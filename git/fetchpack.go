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
// from the remote, and haves to negotiate the missing objects. FetchPack
// always makes a single request and declares "done" at the end.
func FetchPack(c *Client, opts FetchPackOptions, rm Remote, wants []Refname) ([]Ref, error) {
	// We just declare everything we have locally for this remote as a "have"
	// and then declare done, we don't try and be intelligent about what we
	// tell them we have. If we've gotten some objects from another remote,
	// we'll just end up with them duplicated.
	haves, err := rm.GetLocalRefs(c)
	if err != nil {
		return nil, err
	}
	// We put haves into a map to ensure that duplicates are excluded
	havemap := make(map[Sha1]struct{})
	for _, h := range haves {
		havemap[h.Value] = struct{}{}
	}

	conn, err := NewRemoteConn(c, rm)
	if err != nil {
		return nil, err
	}
	if err := conn.OpenConn(); err != nil {
		return nil, err
	}
	defer conn.Close()

	return fetchPackDone(c, opts, conn, wants, havemap)
}

// fetchPackDone makes a single request over conn and declares it done. It returns
// the refs from the connection that were fetched.
func fetchPackDone(c *Client, opts FetchPackOptions, conn RemoteConn, wants []Refname, haves map[Sha1]struct{}) ([]Ref, error) {
	if len(wants) == 0 && !opts.All {
		// There is nothing to fetch, so don't bother doing anything.
		return nil, nil
	}

	if opts.UploadPack != "" {
		if err := conn.SetUploadPack(opts.UploadPack); err != nil {
			return nil, err
		}
	}

	// FIXME: This should be configurable
	conn.SetSideband(os.Stderr)

	var refs []Ref

	// Ref patterns as strings for GetRefs
	var rs []string = make([]string, len(wants))
	for i := range wants {
		rs[i] = string(wants[i])
	}

	switch v := conn.ProtocolVersion(); v {
	case 2:
		log.Println("Using protocol version 2 for fetch-pack")
		capabilities := conn.Capabilities()
		// Discard the extra capabilities advertised by the server
		// because we don't support any of them yet.
		_, ok := capabilities["fetch"]
		if !ok {
			return nil, fmt.Errorf("Server did not advertise fetch capability")
		}
		// First we use ls-refs to get a list of references that we
		// want.
		var rs []string = make([]string, len(wants))
		for i := range wants {
			rs[i] = string(wants[i])
		}
		rmtrefs, err := conn.GetRefs(LsRemoteOptions{Heads: true, Tags: opts.IncludeTag, RefsOnly: true}, rs)
		if err != nil {
			return nil, err
		}
		refs = rmtrefs

		// Now we perform the fetch itself.
		fmt.Fprintf(conn, "command=fetch\n")
		if err := conn.Delim(); err != nil {
			return nil, err
		}
		fmt.Fprintf(conn, "ofs-delta\n")
		if opts.NoProgress {
			fmt.Fprintf(conn, "no-progress\n")
		}
		wanted := false
		for _, ref := range refs {
			have, _, err := c.HaveObject(ref.Value)
			if err != nil {
				return nil, err
			}
			if !have {
				fmt.Fprintf(conn, "want %v\n", ref.Value)
				wanted = true
			}
		}
		if !wanted {
			return nil, fmt.Errorf("Already up to date.")
		}
		for ref := range haves {
			fmt.Fprintf(conn, "have %v\n", ref)
		}
		fmt.Fprintf(conn, "done\n")
		if err := conn.Flush(); err != nil {
			return nil, err
		}
		buf := make([]byte, 65536)
		n, err := conn.Read(buf)
		if err != nil {
			return nil, err
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
		rmtrefs, err := conn.GetRefs(LsRemoteOptions{Heads: true, Tags: true, RefsOnly: true}, rs)
		if err != nil {
			return nil, err
		}
		refs = rmtrefs
		if len(rmtrefs) == 0 {
			return nil, nil
		}
		wanted := false
		for _, ref := range rmtrefs {
			found, _, err := c.HaveObject(ref.Value)
			if err != nil {
				return nil, err
			}
			if found {
				haves[ref.Value] = struct{}{}
				continue
			}

			if !wanted {
				capabilities := conn.Capabilities()
				log.Printf("Server Capabilities: %v\n", capabilities)
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
				caps = strings.TrimSpace(caps)
				log.Printf("Sending capabilities: %v", caps)
				log.Printf("want %v (%v)\n", ref.Value, ref.Name)
				fmt.Fprintf(conn, "want %v %v\n", ref.Value, caps)
				wanted = true
			} else {
				log.Printf("want %v (%v)\n", ref.Value, ref.Name)
				fmt.Fprintf(conn, "want %v\n", ref.Value)
			}
		}
		if !wanted {
			// Nothing wanted, already up to date.
			return refs, nil
		}
		if h, ok := conn.(*smartHTTPConn); ok {
			// Hack so that the flush doesn't send a request.
			h.almostdone = true
		}
		if err := conn.Flush(); err != nil {
			return nil, err
		}
		for ref := range haves {
			log.Printf("have %v\n", ref)
			fmt.Fprintf(conn, "have %v\n", ref)
		}

		if _, err := fmt.Fprintf(conn, "done\n"); err != nil {
			return nil, err
		}

		// Read the last ack/nack and discard it before
		// reading the pack file.
		buf := make([]byte, 65536)
		if _, err := conn.Read(buf); err != nil {
			return nil, err
		}
		if len(haves) > 1 {
			// If there were have lines, read the extras to ensure
			// they're all read before trying to read the packfile.
			for i := 0; i < len(haves); i++ {
				if _, err := conn.Read(buf); err != nil {
					return nil, err
				}
			}
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
	_, err := IndexAndCopyPack(
		c,
		IndexPackOptions{
			Verbose: opts.Verbose,
			FixThin: opts.Thin,
		},
		conn,
	)
	return refs, err
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
		n, err := io.ReadFull(p.conn, buf[0:4])
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
		n, err := io.ReadFull(p.conn, buf[0:4])
		if err != nil {
			return 0, err
		}

		// Allow either flush packets or data with a sideband channel
		if n != 4 {
			return 0, fmt.Errorf("Bad read for git protocol: read %v (%s)", n, buf[:n])
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
			_, err = p.conn.Read(buf[0:1])
			if err != nil {
				return 0, err
			}
			switch buf[0] {
			case sidebandDataChannel:
				return io.ReadFull(p.conn, buf[:size-5])
			case sidebandChannel:
				n, err := io.ReadFull(p.conn, buf[:size-5])
				if err != nil {
					return n, err
				}
				if p.sideband != nil {
					fmt.Fprintf(p.sideband, "remote: %s", buf[:n])
				}
				goto sidebandRead
			case sidebandErrChannel:
				n, err := io.ReadFull(p.conn, buf[:size-5])
				if err != nil {
					return n, err
				}
				return n, fmt.Errorf("%s", buf[:n])
			default:
				return 0, fmt.Errorf("Invalid sideband channel: %d", buf[0])
			}
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
