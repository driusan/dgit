package git

import (
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strings"
)

type Remote string

func (r Remote) RemoteURL(c *Client) (string, error) {
	config, err := LoadLocalConfig(c)
	if err != nil {
		return "", err
	}
	if strings.Index(r.String(), "://") != -1 {
		// It's already a URL
		return string(r), nil
	}
	if File(r.String()).Exists() {
		// It's a known file path, so convert it to a file url
		// and let localConn handle it.
		// It needs to be absolute for the file:// url to work.
		abs, err := filepath.Abs(string(r))
		if err != nil {
			return "", err
		}
		return "file://" + abs, nil
	}
	// If it might be a remote name, look it up in the config.
	cfg, _ := config.GetConfig(fmt.Sprintf("remote.%v.url", r))
	if cfg == "" {
		return "", fmt.Errorf("Unknown remote")
	}
	if File(cfg).Exists() {
		// The config pointed to a path but we already handled
		// that case above, so now try it with the config setting
		abs, err := filepath.Abs(string(cfg))
		if err != nil {
			return "", err
		}
		return "file://" + abs, nil
	}
	return cfg, nil
}

// Returns the URL used to fetch from remote.
func (r Remote) FetchURL(c *Client) (string, error) {
	// FIXME: Handle fetch url config settings if they don't match url
	return r.RemoteURL(c)
}

// Returns the URL used to push to a remote.
func (r Remote) PushURL(c *Client) (string, error) {
	if config := c.GetConfig("remote." + r.String() + ".pushurl"); config != "" {
		return config, nil
	}
	if config := c.GetConfig("remote." + r.String() + ".url"); config != "" {
		return config, nil
	}
	return r.RemoteURL(c)
}

func (r Remote) String() string {
	return string(r)
}

func (r Remote) Name() string {
	return string(r)
}

func (r Remote) IsStateless(c *Client) (bool, error) {
	url, err := r.RemoteURL(c)
	if err != nil {
		return false, err
	}
	url = strings.ToLower(url)
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://"), nil
}

// Returns true if the remote points to a local filesystem path.
func (r Remote) IsFile() bool {
	return strings.HasPrefix(r.String(), "file://") || File(r.String()).Exists()
}

// Gets a list of local references cached for this Remote
func (r Remote) GetLocalRefs(c *Client) ([]Ref, error) {
	allrefs, err := ShowRef(c, ShowRefOptions{}, nil)
	if err != nil {
		return nil, err
	}
	ourrefs := make([]Ref, 0, len(allrefs))
	for _, rf := range allrefs {
		if strings.HasPrefix(rf.Name, "refs/remotes/"+r.String()) {
			ourrefs = append(ourrefs, rf)
		}
	}
	return ourrefs, nil
}

type GitService uint8

const (
	UploadPackService = GitService(iota)
	ReceivePackService
)

// A RemoteConn represends a connection to a remote which communicates
// with the remote.
type RemoteConn interface {
	// Opens a connection to the remote. This requires at least one round
	// trip to the service and may mutate the state of this RemoteConn.
	//
	// After calling this, the RemoteConn should be in a useable state.
	OpenConn(GitService) error

	// Gets a list of references on the remote. If patterns is specified,
	// restrict to refs which match the pattern. If not, return all
	// refs
	GetRefs(opts LsRemoteOptions, patterns []string) ([]Ref, error)

	// Close the underlying connection to this service.
	Close() error

	// Sets the name of git-upload-pack to use for this remote, where
	// applicable. This must be called before OpenConn.
	// When called on a transport type that does not support it (such
	// as the git transport protocol), it will return a nil error. An
	// error indicates that the protocol *should* support the operation
	// but was unable to set the variable.
	SetService(string) error

	// Gets the protocol version that was negotiated during connection
	// opening. Only valid after calling OpenConn.
	ProtocolVersion() uint8

	// Returns the capabilities determined during the initial protocol
	// connection.
	//
	// The first index is the capability, the second is the arguments
	// defined for it.
	Capabilities() map[string]map[string]struct{}

	// Tells the connection to print any sideband data to w
	SetSideband(w io.Writer)

	// A RemoteConn should act as a writter. When written to, it should
	// write to the underlying connection in pkt-line format or directly
	// as per SetWriteMode.
	io.Writer

	// Reading from a RemoteConn should return the data after decoding
	// the line length from a pktline.
	// The behaviour of the read depends on the PackProtocolMode set
	// by SetReadMode
	io.Reader

	// Determines how reading from the connection returns data to the
	// caller.
	SetReadMode(mode PackProtocolMode)

	// Determines how reading from the connection returns data to the
	// caller.
	SetWriteMode(mode PackProtocolMode)

	// Send a flush packet to the connection
	Flush() error

	// Sends a Delimiter packet in protocol V2
	Delim() error
}

func NewRemoteConn(c *Client, r Remote) (RemoteConn, error) {
	urls, err := r.RemoteURL(c)
	if err != nil {
		return nil, err
	}
	uri, err := url.Parse(urls)
	if err != nil {
		return nil, err
	}
	switch uri.Scheme {
	case "http", "https":
		conn := &smartHTTPConn{
			sharedRemoteConn: &sharedRemoteConn{uri: uri},
			giturl:           urls,
		}
		return conn, nil
	case "git":
		conn := &gitConn{
			sharedRemoteConn: &sharedRemoteConn{uri: uri},
		}
		return conn, nil
	case "ssh":
		return &sshConn{
			sharedRemoteConn: &sharedRemoteConn{uri: uri},
		}, nil
	case "file":
		return &localConn{
			sharedRemoteConn: &sharedRemoteConn{uri: uri},
		}, nil
	default:
		return nil, fmt.Errorf("Unsupported remote type for: %v", r)
	}
}

// Helper for implenting things which are shared across all RemoteConn
// implementations
type sharedRemoteConn struct {
	uri             *url.URL
	protocolversion uint8
	capabilities    map[string]map[string]struct{}

	// The remote service to be invoked

	service string
	// References advertised during opening of connection. Only valid
	// for protocol v1
	refs []Ref

	*packProtocolReader

	writemode PackProtocolMode
}

func (r *sharedRemoteConn) SetService(s string) error {
	r.service = s
	return nil
}

func (r *sharedRemoteConn) SetWriteMode(m PackProtocolMode) {
	r.writemode = m
}

func (r sharedRemoteConn) Capabilities() map[string]map[string]struct{} {
	return r.capabilities
}

func (r sharedRemoteConn) ProtocolVersion() uint8 {
	return r.protocolversion
}

type RemoteOptions struct {
	Verbose bool
}

type RemoteAddOptions struct {
	RemoteOptions

	Fetch bool
	// Fetch remote branches (not implemented)

	// Import all tags and associated objects when fetching
	Tags bool

	// Branch(es) to track (not implemented)
	Track string

	// Master branch (not implemented)
	Master string

	// Mirror=[push|fetch]
	// Set up remote as a mirror to push to or fetch from
	// (not implemented)
	Mirror string
}
type RemoteShowOptions struct {
	RemoteOptions

	// Do not query the remote with ls-remote, only show the local cache.
	NoQuery bool
}

type RemoteGetURLOptions struct {
	RemoteOptions
	Push bool
	All  bool
}

func RemoteAdd(c *Client, opts RemoteAddOptions, name, url string) error {
	if name == "" {
		return fmt.Errorf("Missing remote name")
	}
	if url == "" {
		return fmt.Errorf("Missing remote URL")
	}

	configname := fmt.Sprintf("remote.%v.url", name)
	if c.GetConfig(configname) != "" {
		return fmt.Errorf("fatal: remote %v already exists.", name)
	}

	config, err := LoadLocalConfig(c)
	if err != nil {
		return err
	}
	config.SetConfig(configname, url)
	config.SetConfig(
		fmt.Sprintf("remote.%v.fetch", name),
		fmt.Sprintf("+refs/heads/*:refs/remotes/%v/*", name),
	)
	return config.WriteConfig()
}

// Retrieves a list of remotes set up in the local git repository
// for Client c.
func RemoteList(c *Client, opts RemoteOptions) ([]Remote, error) {
	config, err := LoadLocalConfig(c)
	if err != nil {
		return nil, err
	}
	configs := config.GetConfigSections("remote", "")
	remotes := make([]Remote, 0, len(configs))
	for _, cfg := range configs {
		remotes = append(remotes, Remote(cfg.subsection))
	}
	return remotes, nil
}

// Prints the remote named r in the format of "git remote show r" to destination
// w.
func RemoteShow(c *Client, opts RemoteShowOptions, r Remote, w io.Writer) error {
	if !opts.NoQuery {
		return fmt.Errorf("NoQuery is currently required")
	}
	fetchurl, err := r.FetchURL(c)
	if err != nil {
		return err
	}
	pushurl, err := r.PushURL(c)
	if err != nil {
		return err
	}
	headref := "(not queried)"
	if !opts.NoQuery {
		// FIXME: ls-remote needs to parse this properly
		headref = "(not implemented)"
	}
	fmt.Fprintf(w,
		`* remote %v
  Fetch URL: %v
  Push  URL: %v
  HEAD branch: %v
  Remote branches:`, r, fetchurl, pushurl, headref)
	if opts.NoQuery {
		fmt.Fprintf(w, " (status not queried)\n")
	} else {
		fmt.Fprintf(w, "\n")
	}
	refbase := fmt.Sprintf("refs/remotes/%v/", r.Name())
	ForEachRefCallback(c, refbase, func(c *Client, ref Ref) error {
		if opts.NoQuery {
			fmt.Fprintf(w, "\t%v\n", strings.TrimPrefix(string(ref.Name), refbase))
		} else {
			// FIXME: Implement this.
			// This needs to add a status after the ref name and
			// merge with ls-remote too to print new ones
		}
		return nil
	})

	config, err := LoadLocalConfig(c)
	if err != nil {
		return err
	}
	fmt.Fprintln(w, `Local branches configured for 'git pull':`)
	// We go through all branches in the local config file, then print them.
	// We need to know what the longest name is in order to format the
	// printing.
	branchconfigs := config.GetConfigSections("branch", "")

	var branches []struct {
		local, remote string
	}
	var longest int
	for _, branch := range branchconfigs {
		if branch.values["remote"] == r.Name() {
			bname := struct {
				local, remote string
			}{local: branch.subsection}
			if remote, ok := branch.values["merge"]; ok {
				bname.remote = branch.subsection
			} else {
				bname.remote = strings.TrimPrefix(remote, "refs/heads/")
			}
			branches = append(branches, bname)
			if lname := len(bname.local); lname > longest {
				longest = lname
			}
		}
	}
	for _, branch := range branches {
		fmt.Fprintf(w, "\t%-*s\tmerges with remote %s\n", longest, branch.local, branch.remote)
	}
	// FIXME: Figure out where the "Local ref configured for git push" line
	// comes from and add it here.
	return nil

}

// Implements the "git remote get-url" command.
func RemoteGetURL(c *Client, opts RemoteGetURLOptions, r Remote) ([]string, error) {
	if opts.Push {
		u, err := r.PushURL(c)
		if err != nil {
			return nil, err
		}
		return []string{u}, nil
	}
	u, err := r.FetchURL(c)
	if err != nil {
		return nil, err
	}
	return []string{u}, nil
}
