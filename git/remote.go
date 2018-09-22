package git

import (
	"fmt"
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
	// It's a remote name, look it up in the config.
	cfg, _ := config.GetConfig(fmt.Sprintf("remote.%v.url", r))
	if cfg == "" {
		return "", fmt.Errorf("Unknown remote")
	}
	return cfg, nil
}

func (r Remote) String() string {
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

// A RemoteConn represends a connection to a remote which communicates
// with the remote.
type RemoteConn interface {
	// Opens a connection to the remote. This requires at least one round
	// trip to the service and may mutate the state of this RemoteConn.
	//
	// After calling this, the RemoteConn should be in a useable state.
	OpenConn() error

	// Gets a list of references on the remote. If patterns is specified,
	// restrict to refs which match the pattern. If not, return all
	// refs
	GetRefs(opts LsRemoteOptions, patterns []string) ([]Ref, error)
}

func NewRemoteConn(c *Client, r Remote) (RemoteConn, error) {
	if sl, err := r.IsStateless(c); err == nil && sl {
		url, err := r.RemoteURL(c)
		if err != nil {
			return nil, err
		}
		conn := &smartHTTPConn{
			giturl: url,
		}
		return conn, nil
	}
	return nil, fmt.Errorf("Unsupported remote type for: %v", r)
}
