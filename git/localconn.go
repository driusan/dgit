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
	sharedRemoteConn

	// name of the remote upload pack command
	uploadpack string

	stdin  io.ReadCloser
	stdout io.WriteCloser
	cmd    *exec.Cmd
}

func (s *localConn) OpenConn() error {
	var cmd *exec.Cmd
	log.Println("Connecting locally via", s.uri.Path)
	if s.uploadpack == "" {
		cmd = exec.Command("git-upload-pack", s.uri.Path)
	} else {
		cmd = exec.Command(s.uploadpack, s.uri.Path)
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
	s.packProtocolReader = packProtocolReader{conn: s.stdin, state: PktLineMode}

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

func (s *localConn) SetUploadPack(up string) error {
	s.uploadpack = up
	return nil
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
	l, err := PktLineEncodeNoNl(data)
	if err != nil {
		return 0, err
	}
	fmt.Fprintf(s.stdout, "%s", l)
	return len(data), nil
}
