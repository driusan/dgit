package git

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

type SendPackOptions struct {
	All    bool
	DryRun bool
	Force  bool

	Verbose bool
	Thin    bool
	Atomic  bool

	Signed      bool
	ReceivePack string
}

func SendPack(c *Client, opts SendPackOptions, r Remote, refs []Refname) error {
	remoteConn, err := NewRemoteConn(c, r)
	if err != nil {
		return err
	}
	if opts.ReceivePack == "" {
		opts.ReceivePack = "git-receive-pack"
	}
	remoteConn.SetService(opts.ReceivePack)
	if err := remoteConn.OpenConn(ReceivePackService); err != nil {
		return err
	}
	defer remoteConn.Close()

	log.Printf("Send Pack Protocol Version %d Capabilities: %v", remoteConn.ProtocolVersion(), remoteConn.Capabilities())
	remoterefs := make(map[Refname]Sha1)
	localrefs := make(map[Refname]Sha1)
	var refpatterns []string = make([]string, 0, len(refs))
	for _, ref := range refs {
		refpatterns = append(refpatterns, ref.RemoteName().String())

		local := ref.LocalName()
		if local == "" {
			continue
		}
		localsha, err := ref.CommitID(c)
		if err != nil {
			return err
		}
		localrefs[local] = Sha1(localsha)
	}

	remotes, err := remoteConn.GetRefs(LsRemoteOptions{Heads: true, Tags: true, RefsOnly: true}, refpatterns)
	if err != nil {
		return err
	}
	for _, r := range remotes {
		remoterefs[Refname(r.Name)] = r.Value
	}

	for _, ref := range refs {
		log.Printf("Validating if %v is a fast-forward\n", ref)
		remotesha := remoterefs[ref.RemoteName()]
		local := ref.LocalName()
		if local == "" {
			log.Printf("%v is a delete, not a fast-forward\n", ref)
			continue
		}
		if remotesha == (Sha1{}) {
			log.Printf("%v is a new branch\n", ref)
			continue
		}
		localsha := localrefs[ref.LocalName()]
		if localsha == remotesha {
			// Unmodified
			continue
		}

		isancestor := CommitID(remotesha).IsAncestor(c, CommitID(localsha))
		if !isancestor && !opts.Force {
			return fmt.Errorf("Remote %v is not a fast-forward of %v", remotesha, localsha)
		}
		// FIXME: Check if any failed and honour opts.Atomic instead of bothering
		// the server
	}

	if opts.DryRun {
		return nil
	}

	remoteConn.SetWriteMode(PktLineMode)

	// w := os.Stderr
	w := remoteConn
	// Send update lines
	for i, ref := range refs {
		remoteref := remoterefs[ref.RemoteName()]
		localref := localrefs[ref.LocalName()]
		// This handles all of update, delete, and create since the localrefs
		// and remoterefs map will return Sha1{} if the key isn't set
		if i == 0 {
			var caps []string = []string{"ofs-delta", "report-status", "agent=dgit/0.0.2"}
			if opts.Atomic {
				caps = append(caps, "atomic")
			}

			fmt.Fprintf(w, "%v %v %v\000 %v\n", remoteref, localref, ref.RemoteName(), strings.Join(caps, " "))
		} else {
			fmt.Fprintf(w, "%v %v %v\n", remoteref, localref, ref.RemoteName())
		}
	}

	// Figure out what should go in the pack
	var revlistincludes []Commitish = make([]Commitish, 0, len(localrefs))
	for _, sha := range localrefs {
		revlistincludes = append(revlistincludes, CommitID(sha))
	}
	var revlistexcludes []Commitish = make([]Commitish, 0, len(remoterefs))
	for _, sha := range remoterefs {
		if have, _, err := c.HaveObject(sha); have && err == nil {
			revlistexcludes = append(revlistexcludes, CommitID(sha))
		}
	}

	objects, err := RevList(c, RevListOptions{Quiet: true, Objects: true}, nil, revlistincludes, revlistexcludes)
	if err != nil {
		return err
	}

	// Send the pack itself
	remoteConn.SetWriteMode(DirectMode)
	fmt.Fprintf(w, "0000")
	rcaps := remoteConn.Capabilities()

	// Our deltas aren't robust enough to reliably send in the wild yet,
	// so for now don't use them in send-pack. (Our implementation is
	// also slow enough that packing often takes longer than writing the
	// raw data)
	popts := PackObjectsOptions{Window: 0}
	if _, ok := rcaps["ofs-delta"]; ok {
		popts.DeltaBaseOffset = true
	}

	if _, err := PackObjects(c, popts, w, objects); err != nil {
		return err
	}

	if !opts.DryRun {
		// This causes the HTTP request to send when the transport mechanism
		// is the Smart HTTP connection
		if err := remoteConn.Flush(); err != nil {
			return err
		}
		remoteConn.SetReadMode(PktLineMode)
		io.Copy(os.Stderr, remoteConn)

	}
	return nil
}
