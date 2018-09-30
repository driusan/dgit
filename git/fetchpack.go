package git

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
)

type Refname string

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

// FetchPack fetches a packfile from rmt. It uses wants to retrieve the refnames
// from the remote, and haves to negotiate the missing objects.
func FetchPack(c *Client, opts FetchPackOptions, rmt Remote, wants, haves []Refname) error {
	if len(haves) > 0 {
		// This can currently only be used for cloning, not fetching.
		return fmt.Errorf("Incremental fetch is not implemented")
	}
	if len(wants) == 0 && !opts.All {
		// There is nothing to fetch, so don't bother connecting.
		return nil
	}

	conn, err := NewRemoteConn(c, rmt)
	if err != nil {
		return err
	}
	if opts.UploadPack != "" {
		if err := conn.SetUploadPack(opts.UploadPack); err != nil {
			return err
		}
	}

	if err := conn.OpenConn(); err != nil {
		return err
	}
	defer conn.Close()

	// FIXME: This should be configurable
	conn.SetSideband(os.Stderr)
	refs := make([]string, 0, len(wants))
	for _, r := range wants {
		refs = append(refs, string(r))
	}
	switch v := conn.ProtocolVersion(); v {
	case 2:
		log.Println("Using protocol version 2 for fetch-pack")
		capabilities := conn.Capabilities()
		// Discard the extra capabilities advertised by the server
		// because we don't support any of them yet.
		_, ok := capabilities["fetch"]
		if !ok {
			return fmt.Errorf("Server did not advertise fetch capability")
		}
		// First we use ls-refs to get a list of references that we
		// want.
		var refs []Ref
		var rs []string = make([]string, len(wants))
		for i := range wants {
			rs[i] = string(wants[i])
		}
		rmtrefs, err := conn.GetRefs(LsRemoteOptions{Heads: true, Tags: true, RefsOnly: true}, rs)
		if err != nil {
			return err
		}
		refs = rmtrefs
		fmt.Printf("%v", refs)

		// Now we perform the fetch itself.
		fmt.Fprintf(conn, "command=fetch\n")
		if err := conn.Delim(); err != nil {
			return err
		}
		fmt.Fprintf(conn, "ofs-delta\n")
		if opts.NoProgress {
			fmt.Fprintf(conn, "no-progress\n")
		}
		wanted := false
		for _, ref := range refs {
			have, _, err := c.HaveObject(ref.Value)
			if err != nil {
				return err
			}
			if !have {
				fmt.Fprintf(conn, "want %v\n", ref.Value)
				wanted = true
			}
		}
		if !wanted {
			return fmt.Errorf("Already up to date.")
		}
		fmt.Fprintf(conn, "done\n")
		if err := conn.Flush(); err != nil {
			return err
		}
		buf := make([]byte, 65536)
		n, err := conn.Read(buf)
		if err != nil {
			return err
		}
		if string(buf[:n]) != "packfile\n" {
			// Panic because this is a bug in dgit. There are other
			// valid values that a server can return, but we don't
			// support them, so make sure people know it's a bug.
			panic(fmt.Sprintf("Unexpected line returned: got %s want packfile", buf[:n]))
		}

		// V2 always uses side-band-64k
		conn.SetReadMode(PktLineSidebandMode)
	default:
		// protocol v1
		log.Printf("Using protocol was %d: using version 1 for fetch-pack\n", v)
		sideband := false
		rmtrefs, err := conn.GetRefs(LsRemoteOptions{Heads: true, Tags: true, RefsOnly: true}, refs)
		if err != nil {
			return err
		}
		// FIXME: This doesn't seem to be the right references, it's
		// requesting things even with a nil wants
		for i, ref := range rmtrefs {
			if i == 0 {
				capabilities := conn.Capabilities()
				var caps string
				// Add protocol capabilities on the first line
				if _, ok := capabilities["ofs-delta"]; ok {
					caps += " ofs-delta"
				}
				if opts.Quiet {
					if _, ok := capabilities["quiet"]; ok {
						caps += " quiet"
					}
				}
				if opts.NoProgress {
					if _, ok := capabilities["no-progress"]; ok {
						caps += " no-progress"
					}
				}
				if _, ok := capabilities["side-band-64k"]; ok {
					caps += " side-band-64k"
					sideband = true
				} else if _, ok := capabilities["side-band"]; ok {
					caps += " side-band"
					sideband = true
				}
				if _, ok := capabilities["agent"]; ok {
					caps += " agent=dgit/0.0.2"
				}
				log.Println("Sending capabilities: ", caps)
				fmt.Fprintf(conn, "want %v %v\n", ref.Value, strings.TrimSpace(caps))
			} else {
				fmt.Fprintf(conn, "want %v\n", ref.Value)
			}
		}
		if err := conn.Flush(); err != nil {
			return err
		}
		fmt.Fprintf(conn, "done\n")

		// Read the last ack/nack and discard it before
		// reading the pack file.
		buf := make([]byte, 65536)
		if _, err := conn.Read(buf); err != nil {
			return err
		}
		if sideband {
			conn.SetReadMode(PktLineSidebandMode)
		} else {
			conn.SetReadMode(DirectReadMode)
		}
	}

	// Whether we've used V1 or V2, the connection is now returning the
	// packfile upon read, so we want to index it and copy it into the
	// .git directory.
	_, err = IndexAndCopyPack(
		c,
		IndexPackOptions{
			Verbose: opts.Verbose,
			FixThin: opts.Thin,
		},
		conn,
	)
	return err
}

var flushPkt = errors.New("Git protocol flush packet")
var delimPkt = errors.New("Git protocol delimiter packet")

// A PackProtocolMode determines how calling Read on a RemoteConn
// returns data to the caller.
type PackProtocolMode uint8

// Valid PackProtocolModes
const (
	// Directly pass through read requests (used when sending a packfile
	// without
	DirectReadMode = PackProtocolMode(iota)

	// Decode PktLine format and send the decoded data to the caller.
	// (used when negotiating pack files)
	PktLineMode

	// Like PktLineMode, but also read 1 extra byte for determining which
	// sideband channel the data is on. Keeps reading from the connection
	// until something comes in on the main channel, printing any sideband
	// data to sideband.
	PktLineSidebandMode
)

// A packProtocolReader reads from a connection using the git pack
// protocol, decoding lines read from the connection as necessary.
type packProtocolReader struct {
	conn     io.Reader
	state    PackProtocolMode
	sideband io.Writer
}

const (
	sidebandDataChannel = 1
	sidebandChannel     = 2
	sidebandErrChannel  = 3
)

// Reads a line from the underlying connection into buf in a decoded
// format.
func (p packProtocolReader) Read(buf []byte) (int, error) {
	switch p.state {
	case DirectReadMode:
		return p.conn.Read(buf)
	case PktLineMode:
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
	case PktLineSidebandMode:
	sidebandRead:
		n, err := p.conn.Read(buf[0:5])
		if err != nil {
			return 0, err
		}

		// Allow either flush packets or data with a sideband channel
		if n != 4 && n != 5 {
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
			switch buf[4] {
			case sidebandDataChannel:
				return io.ReadFull(p.conn, buf[:size-5])
			case sidebandChannel:
				n, err := io.ReadFull(p.conn, buf[:size-5])
				if err != nil {
					return n, err
				}
				if p.sideband != nil {
					fmt.Fprintf(p.sideband, "%s", buf[:n])
				}
				goto sidebandRead
			case sidebandErrChannel:
				n, err := io.ReadFull(p.conn, buf[:size-5])
				if err != nil {
					return n, err
				}
				return n, fmt.Errorf("%s", buf[:n])
			default:
				return 0, fmt.Errorf("Invalid sideband channel")
			}
			return io.ReadFull(p.conn, buf[:size-5])
		}

	default:
		return 0, fmt.Errorf("Invalid read mode for pack protocol")

	}

}

func (p *packProtocolReader) SetSideband(w io.Writer) {
	p.sideband = w
}

func (p *packProtocolReader) SetReadMode(mode PackProtocolMode) {
	p.state = mode
}
