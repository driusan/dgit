package git

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
)

// A localConn is like an ssh conn, but it communicates locally
// over a pipe rather than running git-upload-pack remotely over
// ssh.
type localConn struct {
	// Add functionality shared amongst all types of remotes
	*sharedRemoteConn

	stdin  io.ReadCloser
	stdout io.WriteCloser
	cmd    *exec.Cmd
}

var _ RemoteConn = &localConn{}

func (s *localConn) OpenConn(srv GitService) error {
	var cmd *exec.Cmd
	log.Println("Connecting locally via", s.uri.Path)
	if s.service == "" {
		cmd = exec.Command(s.service, s.uri.Path)
	} else {
		switch srv {
		case UploadPackService:
			cmd = exec.Command("git-upload-pack", s.uri.Path)
		case ReceivePackService:
			cmd = exec.Command("git-receive-pack", s.uri.Path)
		default:
			return fmt.Errorf("Unhandled service")
		}
	}
	cmd.Stderr = os.Stderr
	cmdIn, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	s.stdout = cmdIn
	cmdOut, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	s.stdin = cmdOut

	// We don't check error on setenv because if it failed we'll just fall
	// back on protocol v1
	os.Setenv("GIT_PROTOCOL", "version=2")

	if err := cmd.Start(); err != nil {
		return err
	}

	v, cap, refs, err := parseRemoteInitialConnection(s.stdin, false)
	if err != nil {
		s.stdin.Close()
		s.stdout.Close()
		return cmd.Wait()
	}
	s.cmd = cmd
	s.packProtocolReader = &packProtocolReader{conn: s.stdin, state: PktLineMode}

	s.protocolversion = v
	s.capabilities = cap
	s.refs = refs
	return nil
}

func (s localConn) Close() error {
	fmt.Fprintf(s.stdout, "0000")
	s.stdout.Close()
	s.stdin.Close()
	return s.cmd.Wait()
}

func (s localConn) GetRefs(opts LsRemoteOptions, patterns []string) ([]Ref, error) {
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

func (s localConn) Flush() error {
	fmt.Fprintf(s.stdout, "0000")
	return nil
}

func (s localConn) Delim() error {
	fmt.Fprintf(s.stdout, "0001")
	return nil
}

func (s localConn) Write(data []byte) (int, error) {
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
