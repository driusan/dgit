package git

import (
	"fmt"
	"io"
	"net"
	"net/url"
)

// gitConn represents a remote which uses the git protocol
// for transport (ie. a remote that starts with git://)
type gitConn struct {
	uri  *url.URL
	conn io.ReadWriteCloser

	protocolversion uint8
	capabilities    map[string]struct{}

	// References advertised upon opening the connection. Only for
	// protocol v1
	refs []Ref
}

func (g *gitConn) OpenConn() error {
	host := g.uri.Hostname()
	port := g.uri.Port()
	if port == "" {
		port = "9418"
	}

	// Advertisement string. Built it before trying to open the connection
	// just in case something (somehow) goes wrong.
	adv, err := PktLineEncodeNoNl(
		[]byte(fmt.Sprintf(
			"git-upload-pack %s\x00host=%s\x00\x00version=2\x00",
			g.uri.Path,
			host,
		)),
	)
	if err != nil {
		return err
	}
	conn, err := net.Dial("tcp", host+":"+port)
	if err != nil {
		return err
	}
	g.conn = conn

	fmt.Fprintf(conn, "%v", adv)
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
	fmt.Fprintf(g.conn, "0000")
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
		r := packProtocolReader{g.conn}
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
