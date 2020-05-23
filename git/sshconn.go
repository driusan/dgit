package git

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/user"

	"golang.org/x/crypto/ssh"
)

// an sshConn represents a remote which uses ssh
// for transport (ie. a remote that starts with ssh://)
type sshConn struct {
	// Add functionality shared amongst all types of remotes
	*sharedRemoteConn

	session *ssh.Session

	stdin  io.Reader
	stdout io.Writer
}

var _ RemoteConn = &sshConn{}

func (s *sshConn) OpenConn(srv GitService) error {
	host := s.uri.Hostname()
	port := s.uri.Port()
	if port == "" {
		port = "22"
	}
	log.Println("Opening ssh connection to", host, " port", port)

	var username string
	if s.uri.User != nil {
		username = s.uri.User.Username()
	} else {
		u, err := user.Current()
		if err != nil {
			return err
		}
		username = u.Username
	}
	log.Println("Using username", username)
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeysCallback(getSigners),
		},
		HostKeyCallback: hostKeyCallback(),
	}
	conn, err := ssh.Dial("tcp", host+":"+port, config)
	if err != nil {
		return err
	}
	session, err := conn.NewSession()
	if err != nil {
		return err
	}
	session.Stderr = os.Stderr
	s.stdin, session.Stdout = io.Pipe()
	session.Stdin, s.stdout = io.Pipe()
	// We don't check error on setenv because if it failed we'll just fall
	// back on protocol v1
	session.Setenv("GIT_PROTOCOL", "version=2")
	s.session = session

	if err := session.Start(s.service + " " + s.uri.Path); err != nil {
		return err
	}

	v, cap, refs, err := parseRemoteInitialConnection(s.stdin, false)
	if err != nil {
		session.Close()
		return err
	}
	s.packProtocolReader = &packProtocolReader{conn: s.stdin, state: PktLineMode}

	s.protocolversion = v
	s.capabilities = cap
	s.refs = refs
	return nil
}

func (s sshConn) Close() error {
	fmt.Fprintf(s.stdout, "0000")
	return s.session.Close()
}

func (s sshConn) GetRefs(opts LsRemoteOptions, patterns []string) ([]Ref, error) {
	switch s.protocolversion {
	case 1:
		return getRefsV1(s.refs, opts, patterns)
	case 2:
		cmd, err := buildLsRefsCmdV2(opts, patterns)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(s.stdout, cmd)
		line := make([]byte, 65536)
		var vals []Ref
		for {
			n, err := s.Read(line)
			switch err {
			case flushPkt:
				return vals, nil
			case nil: // Nothing
			default:
				return nil, err
			}
			refstr := string(line[0:n])
			ref, err := parseLsRef(refstr)
			if err != nil {
				return nil, err
			}
			vals = append(vals, ref)
		}
	default:
		return nil, fmt.Errorf("Protocol version not supported")
	}
}

func (s sshConn) Flush() error {
	fmt.Fprintf(s.stdout, "0000")
	return nil
}

func (s sshConn) Delim() error {
	fmt.Fprintf(s.stdout, "0001")
	return nil
}

func (s sshConn) Write(data []byte) (int, error) {
	switch s.writemode {
	case PktLineMode:
		l, err := PktLineEncodeNoNl(data)
		if err != nil {
			return 0, err
		}
		fmt.Fprintf(s.stdout, "%s", l)
		return len(data), nil
	case DirectMode:
		return s.stdout.Write(data)
	default:
		return 0, fmt.Errorf("Invalid write mode")
	}
}

// this should be overridden for various platforms. Plan9/9front should parse
// $home/lib/sshthumbs, unix should parse ~/.ssh/known_hosts, and Windows should..
// do something?
func hostKeyCallback() ssh.HostKeyCallback {
	fmt.Fprintln(os.Stderr, "WARNING: Fingerprint for hostname not validated.")
	return ssh.InsecureIgnoreHostKey()
}
