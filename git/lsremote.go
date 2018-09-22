package git

import ()

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
	remoteconn, err := NewRemoteConn(c, r)
	if err != nil {
		return nil, err
	}
	if err := remoteconn.OpenConn(); err != nil {
		return nil, err
	}
	return remoteconn.GetRefs(opts, patterns)
}
