package git

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/user"
	//	"net"
	"net/url"

	"bitbucket.org/mischief/libauth"
	"golang.org/x/crypto/ssh"
)

// an sshConn represents a remote which uses ssh
// for transport (ie. a remote that starts with ssh://)
type sshConn struct {
	// name of the remote upload pack command
	uploadpack string
	uri        *url.URL

	session *ssh.Session

	protocolversion uint8
	capabilities    map[string]struct{}

	// References advertised upon opening the connection. Only for
	// protocol v1
	refs []Ref

	stdin  io.Reader
	stdout io.Writer
}

func (s *sshConn) OpenConn() error {
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
		// FIXME: This should parse the OS appropriate host key digests
		// (ie $home/lib/sshthumbs on Plan9, $HOME/.ssh/known_hosts on
		// other platforms)
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
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

	if err := session.Start(s.uploadpack + " " + s.uri.Path); err != nil {
		return err
	}
	v, cap, refs, err := parseRemoteInitialConnection(s.stdin, false)
	if err != nil {
		session.Close()
		return err
	}
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
		r := packProtocolReader{s.stdin}
		for {
			n, err := r.Read(line)
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

// from mischief's scpu, get a list of signers
func getSigners() ([]ssh.Signer, error) {
	// FIXME: Don't assume Plan 9/factotum is present, look into ~/.ssh/
	// on other platforms.
	k, err := libauth.Listkeys()
	if err != nil {
		// if libauth returned an error, it just means factotum isn't
		// present
		return nil, nil
	}
	signers := make([]ssh.Signer, len(k))
	for i, key := range k {
		s, err := ssh.NewSignerFromKey(&key)
		if err != nil {
			return nil, err
		}
		signers[i] = s
	}
	return signers, nil
}
