package git

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// A smartHTTPConn implements the RemoteConn interface over the smart http
// protocol. It keeps track of state such as "what protocol version did the
// server initially send" and "what info have we already determined from previous
// requests"
type smartHTTPConn struct {
	*sharedRemoteConn

	// The giturl is the URL for this connection. It may or may not be
	// the same as what it was initiated as, depending on whether or not
	// ".git" had to be appended to the URL.
	giturl string

	// username/password to use over HTTP basic auth.
	username, password string

	// nil we haven't tried to open yet, true if successfully got initial
	// git-upload-pack response, and false if there was a problem getting
	// the upload-pack response
	isopen *bool

	// Writing to a smartHTTPConn builds the request in a buffer until a
	// flush packet. Once a flush packet is sent, it sends the request
	// from the buffer
	buf strings.Builder

	// Body of the last response from the server.
	lastresp io.ReadCloser

	almostdone bool
}

// Opens a connection to s.giturl over the smart http protocol
func (s *smartHTTPConn) OpenConn() error {
	// Try directly accessing the server's URL.
	s.giturl = strings.TrimSuffix(s.giturl, "/")
	expectedmime := "application/x-git-upload-pack-advertisement"

	// Make variable references out of true and false so we can take
	// their address for isopen
	var trueref bool = true
	var falseref bool = false

	req, err := http.NewRequest("GET", s.giturl+"/info/refs?service=git-upload-pack", nil)
	if err != nil {
		// We couldn't even try making a request, so give up early.
		return err
	}

	// Set username and password if applicable
	if s.username != "" || s.password != "" {
		req.SetBasicAuth(s.username, s.password)
	}
	req.Header.Set("Git-Protocol", "version=2")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// If we couldn't perform the request, there's probably a
		// network issue so give up.
		return err
	}
	defer resp.Body.Close()
	var respreader io.Reader
	if ct := resp.Header.Get("Content-Type"); ct != expectedmime || resp.StatusCode != 200 {
		// If the content-type was wrong, try again at "url.git"
		log.Printf("Unexpected Content-Type for %v: got %v\n", s.giturl, ct)
		s.giturl = s.giturl + ".git"
		req, err = http.NewRequest("GET", s.giturl+"/info/refs?service=git-upload-pack", nil)
		if s.username != "" || s.password != "" {
			req.SetBasicAuth(s.username, s.password)
		}
		req.Header.Set("Git-Protocol", "version=2")
		newresp, err := http.DefaultClient.Do(req)
		if err != nil {
			s.isopen = &falseref
			return fmt.Errorf("Could not connect to remote")
		}
		defer newresp.Body.Close()
		if ct := newresp.Header.Get("Content-Type"); ct != expectedmime || newresp.StatusCode != 200 {
			log.Printf("Unexpected Content-Type for %v: got %v\n", s.giturl, ct)
			s.isopen = &falseref
			return fmt.Errorf("Remote did not speak git protocol")
		}
		respreader = newresp.Body
	} else {
		respreader = resp.Body
	}

	version, capabilities, refs, err := parseRemoteInitialConnection(respreader, true)
	if err != nil {
		s.isopen = &falseref
		return err
	}
	switch version {
	case 1, 2:
		s.packProtocolReader = &packProtocolReader{respreader, PktLineMode, nil, nil}
		s.protocolversion = version
		s.capabilities = capabilities
		s.refs = refs
		s.isopen = &trueref
		log.Printf("Using protocol version %d. Capabilities: %v\n", version, capabilities)
		// The initial connection has been made, we now know the capabilities
		// of the remote and can act appropriately.
		return nil
	default:
		return fmt.Errorf("Unsupported protocol version")
	}
}

func parseRemoteInitialConnection(r io.Reader, stateless bool) (uint8, map[string]map[string]struct{}, []Ref, error) {
	switch line := loadLine(r); line {
	case "version 2", "version 2\n":
		cap := make(map[string]map[string]struct{})
		for line := loadLine(r); line != ""; line = loadLine(r) {
			// Version 2 lists capabilities one per line. If there's
			// an equal sign, it's the options supported by that
			// command.
			if eq := strings.Index(line, "="); eq == -1 {
				cap[line] = make(map[string]struct{})
			} else {
				name := line[:eq]

				args := make(map[string]struct{})
				for _, opt := range strings.Fields(line[eq+1:]) {
					args[opt] = struct{}{}
				}
				cap[name] = args
			}
		}
		return 2, cap, nil, nil

	case "# service=git-upload-pack", "# service=git-upload-pack\n":
		// An http connection starts with the service announcement. We
		// need to parse the next line
		line = loadLine(r)
		if line == "" {
			line = loadLine(r)
		}
		fallthrough
	default:
		cap := make(map[string]map[string]struct{})
		var refs []Ref
		// parseLine populates cap if it's the first line, and
		// refs otherwise
		var parseLine = func(s string) (*Ref, error) {
			log.Println(s)
			var ret Ref
			// The first space (ie the end of the Sha1)
			var firstSpace int
			// The end of the name position (either \0 or \n
			//  can cause a name to end)
			var nameEnd int
			for idx, char := range s {
				if char == ' ' && ret.Value == (Sha1{}) {
					sha1, err := Sha1FromString(s[0:idx])
					if err != nil {
						return nil, err
					}
					ret.Value = sha1
					firstSpace = idx
				}
				if char == '\000' {
					// The first line. If it's the first 0
					// we encountered, set name, otherwise
					// set nameEnd to the next char for parsing
					// the capabilities
					nameEnd = idx + 1
					if ret.Name == "" {
						ret.Name = s[firstSpace+1 : idx]
					}
				}
				if char == '\n' {
					if ret.Name == "" {
						// Not the first line, so there
						// was no \0
						ret.Name = s[firstSpace+1 : idx]
						return &ret, nil
					}
					// The first line, so parse the capabilities
					caps := strings.Split(s[nameEnd:], " ")
					for _, c := range caps {
						if eq := strings.Index(c, "="); eq == -1 {
							cap[c] = make(map[string]struct{})
						} else {
							name := line[:eq]
							args := make(map[string]struct{})
							for _, opt := range strings.Fields(line[eq+1:]) {
								args[opt] = struct{}{}
							}
							cap[name] = args
						}
					}
					return &ret, nil
				}
			}
			return &ret, nil
		}
		ref, err := parseLine(line)
		if err != nil {
			return 0, nil, nil, err
		}
		if ref != nil {
			refs = append(refs, *ref)
		}

		for line := loadLine(r); line != ""; line = loadLine(r) {

			ref, err := parseLine(line)
			if err != nil {
				return 0, nil, nil, err
			}
			if ref != nil {
				refs = append(refs, *ref)
			}
		}
		return 1, cap, refs, nil
	}
}

func (s smartHTTPConn) GetRefs(opts LsRemoteOptions, patterns []string) ([]Ref, error) {
	if s.isopen == nil || *s.isopen == false {
		return nil, fmt.Errorf("Connection is not open")
	}
	switch s.protocolversion {
	case 1:
		return getRefsV1(s.refs, opts, patterns)
	case 2:
		var vals []Ref
		// Make request to $giturl/git-upload-pack
		// payload ls-refs=	maybe symrefs, maybe peel, maybe ref-prefix depending on options\n
		// foreach pattern 0001 pktline patternarg
		// 0000
		topost, err := buildLsRefsCmdV2(opts, patterns)
		if err != nil {
			return nil, err
		}
		r, err := http.NewRequest("POST", s.giturl+"/git-upload-pack", strings.NewReader(topost))
		r.Header.Set("User-Agent", "dgit/0.0.2")
		r.Header.Set("Git-Protocol", "version=2")
		r.Header.Set("Content-Type", "application/x-git-upload-pack-request")
		r.ContentLength = int64(len([]byte(topost)))
		resp, err := http.DefaultClient.Do(r)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		for line := loadLine(resp.Body); line != ""; line = loadLine(resp.Body) {
			ref, err := parseLsRef(string(line))
			if err != nil {
				return nil, err
			}
			vals = append(vals, ref)
		}
		return vals, nil

	default:
		return nil, fmt.Errorf("Unsupported protocol version")
	}
}

func (s smartHTTPConn) Close() error {
	// http remotes are stateless, closing them is meaningless
	return nil
}

func (s smartHTTPConn) SetUploadPack(string) error {
	// Not applicable for http (or should it change the URL?)
	return nil
}

func (s *smartHTTPConn) Write(data []byte) (int, error) {
	l, err := PktLineEncodeNoNl(data)
	if err != nil {
		return 0, err
	}
	if n, err := fmt.Fprintf(&s.buf, "%s", l); err != nil {
		return n, err
	}
	// protocol v2 sends "done" and then a flush, while v1 sends a flush
	// and then done. So if it's v2, don't send the request when we see
	// "done"
	if s.protocolversion != 2 {
		if string(data) == "done\n" || string(data) == "done" {
			if err := s.sendRequest("application/x-git-upload-pack-result"); err != nil {
				return 0, err
			}
			s.almostdone = false
		}
	}
	return len(l), nil
}
func (s *smartHTTPConn) Flush() error {
	fmt.Fprintf(&s.buf, "0000")
	if s.almostdone && s.protocolversion == 1 {
		return nil
	}
	return s.sendRequest("application/x-git-upload-pack-result")
}

func (s *smartHTTPConn) Delim() error {
	fmt.Fprintf(&s.buf, "0001")
	return nil
}

func (s *smartHTTPConn) sendRequest(expectedmime string) error {
	log.Println("Sending HTTP Request")
	topost := s.buf.String()
	r, err := http.NewRequest("POST", s.giturl+"/git-upload-pack", strings.NewReader(topost))
	r.Header.Set("User-Agent", "dgit/0.0.2")
	if s.protocolversion == 2 {
		r.Header.Set("Git-Protocol", "version=2")
	}

	r.Header.Set("Content-Type", "application/x-git-upload-pack-request")
	r.ContentLength = int64(len([]byte(topost)))
	if s.username != "" || s.password != "" {
		r.SetBasicAuth(s.username, s.password)
	}
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return err
	}
	// If the response or status code is wrong we return an
	// error and don't try alternatives, because this should
	// have all been negotiated correctly during OpenConn()
	if ct := resp.Header.Get("Content-Type"); ct != expectedmime {
		return fmt.Errorf("Unexpected Content-Type for %v: got %v\n", s.giturl, ct)
	}
	if sc := resp.StatusCode; sc != 200 {
		return fmt.Errorf("Unexpected status code for response: got %v", sc)
	}

	s.lastresp = resp.Body
	fmt.Printf("Setting connection")
	s.packProtocolReader.conn = s.lastresp
	return nil
}
func (s *smartHTTPConn) Read(buf []byte) (int, error) {
	if s.isopen == nil || *s.isopen == false {
		return 0, fmt.Errorf("Connection not open")
	}
	if s.lastresp == nil {
		return 0, fmt.Errorf("Can not read until after first Flush() call")
	}
	return s.sharedRemoteConn.packProtocolReader.Read(buf)
}

func getRefsV1(refs []Ref, opts LsRemoteOptions, patterns []string) ([]Ref, error) {
	var vals []Ref
	all := !(opts.Heads || opts.Tags)
refs:
	for _, r := range refs {
		if !all {
			if opts.Heads {
				if strings.HasPrefix(r.Name, "refs/heads/") || r.Name == "HEAD" {
					goto good
				}
			}
			if opts.Tags {
				if strings.HasPrefix(r.Name, "refs/tags/") {
					goto good
				}
			}
			// We weren't looking for all and it was neither a tag nor
			// a head ref
			continue refs
		}
	good:
		if len(patterns) == 0 {
			vals = append(vals, r)
			continue refs
		}
		for _, p := range patterns {
			// FIXME: Matches is the logic used by show-ref
			// This isn't the same as the logic for ls-remote
			// which should honour ie. "Foo*"
			if r.Matches(p) {
				vals = append(vals, r)
				continue refs
			}
		}
	}
	return vals, nil
}

// Builds the ls-refs command to send over V2, for both stateless and stateful
// protocol variants
func buildLsRefsCmdV2(opts LsRemoteOptions, patterns []string) (string, error) {
	// FIXME: This should take a writer instead of returning a string
	cmd, err := PktLineEncode([]byte("command=ls-refs"))
	if err != nil {
		return "", err
	}
	cmd += "0001"
	if !opts.RefsOnly {
		penc, err := PktLineEncode([]byte("peel"))
		if err != nil {
			return "", err
		}
		cmd += penc
	}
	if opts.SymRef {
		penc, err := PktLineEncode([]byte("symrefs"))
		if err != nil {
			return "", err
		}
		cmd += penc
	}
	if len(patterns) == 0 {
		if opts.Heads {
			penc, err := PktLineEncode([]byte("ref-prefix refs/heads"))
			if err != nil {
				return "", err
			}
			cmd += penc

		}
		if opts.Tags {
			penc, err := PktLineEncode([]byte("ref-prefix refs/tags"))
			if err != nil {
				return "", err
			}
			cmd += penc

		}
	} else {
		// These appear to be the prefixes git uses with
		// tracing turned on. They don't seem to change
		// regardless of the command line options.
		prefixes := []string{"", "refs/", "refs/heads/", "refs/tags/", "refs/remotes/"}
		// FIXME: Do better
		for _, p := range patterns {
			for _, prefix := range prefixes {
				penc, err := PktLineEncode([]byte("ref-prefix " + prefix + p))
				if err != nil {
					return "", err
				}
				cmd += penc
			}
		}
	}
	return cmd.String() + "0000", nil
}

// parses a ref returned from the LsRefs command
func parseLsRef(s string) (Ref, error) {
	sha1, err := Sha1FromString(s[0:40])
	if err != nil {
		return Ref{}, err
	}
	name := string(s[41:])
	return Ref{Name: name, Value: sha1}, nil
}
