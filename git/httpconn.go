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
	// The giturl is the URL for this connection. It may or may not be
	// the same as what it was initiated as, depending on whether or not
	// ".git" had to be appended to the URL.
	giturl string

	// username/password to use over HTTP basic auth.
	username, password string

	// protocol 1 or 2
	protocolversion uint8

	// capabilities advertised in the initial connection
	capabilities map[string]struct{}

	// nil we haven't tried to open yet, true if successfully got initial
	// git-upload-pack response, and false if there was a problem getting
	// the upload-pack response
	isopen *bool

	// List of refs advertised at the connection opening. Only valid for
	// protocol version 1. Version 2 uses the ls-refs command.
	refs []Ref
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

	version, capabilities, refs, err := parseRemoteInitialConnection(respreader)
	if err != nil {
		s.isopen = &falseref
		return err
	}
	switch version {
	case 1, 2:
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

func parseRemoteInitialConnection(r io.Reader) (uint8, map[string]struct{}, []Ref, error) {
	switch line := loadLine(r); line {
	case "version 2":
		cap := make(map[string]struct{})
		for line := loadLine(r); line != ""; line = loadLine(r) {
			// Version 2 lists capabilities one per line and nothing
			// else on the initial connection
			cap[line] = struct{}{}
		}
		return 2, cap, nil, nil
	case "# service=git-upload-pack\n":
		// version 1 has the service advertisement, a flush line,
		// and then a list of refs. The first ref has a list of
		// capabilities hidden behind a nil byte.
		if loadLine(r) != "" {
			// Flush line
			return 0, nil, nil, InvalidResponse
		}

		cap := make(map[string]struct{})
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
						ret.Name = s[firstSpace:idx]
					}
				}
				if char == '\n' {
					if ret.Name == "" {
						// Not the first line, so there
						// was no \0
						ret.Name = s[firstSpace:idx]
						return &ret, nil
					}
					// The first line, so parse the capabilities
					caps := strings.Split(s[nameEnd:], " ")
					for _, c := range caps {
						cap[c] = struct{}{}
					}
					return &ret, nil
				}
			}
			return &ret, nil
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
	default:
		return 0, nil, nil, fmt.Errorf("Unhandled first response line: '%v'", line)
	}
}

func (s smartHTTPConn) GetRefs(opts LsRemoteOptions, patterns []string) ([]Ref, error) {
	if s.isopen == nil || *s.isopen == false {
		return nil, fmt.Errorf("Connection is not open: %v", s)
	}
	var vals []Ref
	switch s.protocolversion {
	case 1:
		if len(patterns) == 0 {
			return s.refs, nil
		}

	refs:
		for _, r := range s.refs {
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
	case 2:
		// Make request to $giturl/git-upload-pack
		// payload ls-refs=	maybe symrefs, maybe peel, maybe ref-prefix depending on options\n
		// foreach pattern 0001 pktline patternarg
		// 0000
		topost, err := PktLineEncode([]byte("command=ls-refs"))
		topost += "0001"
		if !opts.RefsOnly {
			penc, err := PktLineEncode([]byte("peel"))
			if err != nil {
				return nil, err
			}
			topost += penc
		}
		if opts.SymRef {
			penc, err := PktLineEncode([]byte("symrefs"))
			if err != nil {
				return nil, err
			}
			topost += penc
		}
		if len(patterns) == 0 {
			if opts.Heads {
				penc, err := PktLineEncode([]byte("ref-prefix refs/heads"))
				if err != nil {
					return nil, err
				}
				topost += penc

			}
			if opts.Tags {
				penc, err := PktLineEncode([]byte("ref-prefix refs/tags"))
				if err != nil {
					return nil, err
				}
				topost += penc

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
						return nil, err
					}
					topost += penc
				}
			}
		}

		topost += "0000"
		if err != nil {
			return nil, err
		}
		r, err := http.NewRequest("POST", s.giturl+"/git-upload-pack", strings.NewReader(topost.String()))
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
			sha1, err := Sha1FromString(line[0:40])
			if err != nil {
				return nil, err
			}
			name := line[41:]
			vals = append(vals, Ref{Name: name, Value: sha1})
		}
		return vals, nil

	default:
		return nil, fmt.Errorf("Unsupported protocol version: %v", s.protocolversion)
	}
}
