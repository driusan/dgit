package git

import (
	"fmt"
	"os"
)

type LsRemoteOptions struct {
	Heads, Tags   bool
	RefsOnly      bool
	Quiet         bool
	UploadPack    string
	ExitCode      bool
	GetURL        bool
	SymRef        bool
	Sort          string
	ServerOptions []string
}

func LsRemote(c *Client, opts LsRemoteOptions, r Remote, patterns []string) ([]Ref, error) {
	if r == "" {
		r = "origin"
		if !opts.Quiet {
			rurl := c.GetConfig("remote.origin.url")
			if rurl == "" {
				return nil, fmt.Errorf("Can not ls-remote")
			}
			fmt.Fprintln(os.Stderr, "From", rurl)
		}
	}

	remoteconn, err := NewRemoteConn(c, r)
	if err != nil {
		return nil, err
	}
	if opts.UploadPack == "" {
		opts.UploadPack = "git-upload-pack"
	}
	remoteconn.SetService(opts.UploadPack)

	if err := remoteconn.OpenConn(UploadPackService); err != nil {
		return nil, err
	}
	defer remoteconn.Close()
	return remoteconn.GetRefs(opts, patterns)
}
