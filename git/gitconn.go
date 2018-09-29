package git

import (
	"fmt"
	"io"
	"net"
)

// gitConn represents a remote which uses the git protocol
// for transport (ie. a remote that starts with git://)
type gitConn struct {
	sharedRemoteConn
	conn io.ReadWriteCloser
}

func (g *gitConn) OpenConn() error {
	host := g.uri.Hostname()
	port := g.uri.Port()
	if port == "" {
		port = "9418"
	}

	conn, err := net.Dial("tcp", host+":"+port)
	if err != nil {
		return err
	}
	g.conn = conn
	g.packProtocolReader = packProtocolReader{g.conn, PktLineMode, nil}

	// Advertise the connection and try to negotiate protocol version 2
	fmt.Fprintf(
		g,
		"git-upload-pack %s\x00host=%s\x00\x00version=2\x00",
		g.uri.Path,
		host,
	)

	v, cap, refs, err := parseRemoteInitialConnection(conn, false)
	if err != nil {
		conn.Close()
		return err
	}
	g.protocolversion = v
	g.capabilities = cap
	g.refs = refs
	return nil
}

func (g gitConn) Close() error {
	g.Flush()
	return g.conn.Close()
}

func (g gitConn) GetRefs(opts LsRemoteOptions, patterns []string) ([]Ref, error) {
	switch g.protocolversion {
	case 1:
		return getRefsV1(g.refs, opts, patterns)
	case 2:
		cmd, err := buildLsRefsCmdV2(opts, patterns)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(g.conn, cmd)
		line := make([]byte, 65536)
		var vals []Ref
		for {
			n, err := g.Read(line)
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

func (g gitConn) SetUploadPack(up string) error {
	// not applicable for git protocol
	return nil
}

func (g gitConn) Write(data []byte) (int, error) {
	l, err := PktLineEncodeNoNl(data)
	if err != nil {
		return 0, err
	}
	fmt.Fprintf(g.conn, "%s", l)
	fmt.Printf("%s", l)
	// We lie about how much data was written since
	// we wrote more than asked.
	return len(data), nil
}

func (g gitConn) Flush() error {
	fmt.Fprintf(g.conn, "0000")
	return nil
}

func (g gitConn) Delim() error {
	fmt.Fprintf(g.conn, "0001")
	return nil
}
