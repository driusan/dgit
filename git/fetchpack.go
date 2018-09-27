package git

import (
	"errors"
	"fmt"
	"io"
	"strconv"
)

type FetchPackOptions struct {
	All                            bool
	Stdin                          io.Reader
	StatelessRPC                   bool
	Quiet                          bool
	Keep                           bool
	Thin                           bool
	IncludeTag                     bool
	UploadPack                     string
	Depth                          int32
	DeepenRelative                 bool
	NoProgress                     bool
	CheckSelfContainedAndConnected bool
	Verbose                        bool
}

func FetchPack(c *Client, opts FetchPackOptions, rmt Remote, refs []Ref) error {
	return fmt.Errorf("fetch-pack not implemented")
}

var flushPkt = errors.New("Git protocol flush packet")
var delimPkt = errors.New("Git protocol delimiter packet")

// A packProtocolReader reads from a connection using the git pack
// protocol, decoding lines read from the connection as necessary.
type packProtocolReader struct {
	conn io.Reader
}

// Reads a line from the underlying connection into buf in a decoded
// format.
func (p packProtocolReader) Read(buf []byte) (int, error) {
	n, err := p.conn.Read(buf[0:4])
	if err != nil {
		return 0, err
	}
	if n != 4 {
		return 0, fmt.Errorf("Bad read for git protocol")
	}
	switch string(buf[0:4]) {
	case "0000":
		// Denotes a boundary between client/server
		// communication
		return 0, flushPkt
	case "0001":
		// Delimits a command in protocol v2
		return 0, delimPkt
	default:
		size, err := strconv.ParseUint(string(buf[0:4]), 16, 0)
		if err != nil {
			return 0, err
		}
		return io.ReadFull(p.conn, buf[:size-4])
	}
}
