package git

import (
	"fmt"
	"strings"
)

type CloneOptions struct {
	InitOptions
	FetchPackOptions
	Local                      bool
	NoHardLinks                bool
	Reference, ReferenceIfAble bool
	Dissociate                 bool
	Progress                   bool
	NoCheckout                 bool
	Mirror                     bool
	// use name instead of origin as upstream remote.
	Origin string
	// Use branch instead of HEAD as default branch to checkout
	Branch string
	// Set configs in the newly created repository's config.
	Configs map[string]string

	// Only clone a single branch (either HEAD or Branch option)
	SingleBranch bool

	NoTags            bool
	RecurseSubmodules bool
	ShallowSubmodules bool
	Jobs              int
}

// Clones a new repository from rmt into the directory dst, which must
// not already exist.
func Clone(opts CloneOptions, rmt Remote, dst File) error {
	// This basically does the following:
	// 1. Verify preconditions
	// 2. Init
	// 3. Fetch-pack --all
	// 4. Set up some default config variables
	// 5. UpdateRef master
	// 6. Reset --hard

	if dst.Exists() {
		return fmt.Errorf("Directory %v already exists, can not clone.\n", dst)
	}
	c, err := Init(nil, opts.InitOptions, dst.String())
	if err != nil {
		return err
	}

	opts.FetchPackOptions.All = true
	opts.FetchPackOptions.Verbose = true

	conn, err := NewRemoteConn(c, rmt)
	if err != nil {
		return err
	}
	if err := conn.OpenConn(); err != nil {
		return err
	}
	defer conn.Close()

	refs, err := fetchPackDone(c, opts.FetchPackOptions, conn, nil, nil)
	if err != nil {
		return err
	}
	config, err := LoadLocalConfig(c)
	config.SetConfig("remote.origin.url", rmt.String())
	br := opts.Branch
	if br == "" {
		br = "master"
	}
	org := opts.Origin
	if opts.Origin == "" {
		org = "origin"
	}
	config.SetConfig(fmt.Sprintf("remote.%v.url", org), rmt.String())
	config.SetConfig(fmt.Sprintf("remote.%v.remote", br), org)
	// This should be smarter and get the HEAD symref from the connection.
	// It isn't necessarily named refs/heads/master
	config.SetConfig(fmt.Sprintf("remote.%v.merge", br), "refs/heads/master")
	if err := config.WriteConfig(); err != nil {
		return err
	}
	for _, ref := range refs {
		if !strings.HasPrefix(ref.Name, "refs/heads/") {
			// FIXME: This should have been done by GetRefs()
			continue
		}
		brname := strings.TrimPrefix(ref.Name, "refs/heads/")
		f := c.GitDir.File(File("refs/remotes/" + org + "/" + brname))
		if err := f.Create(); err != nil {
			return err
		}
		if err := f.Append(fmt.Sprintf("%v\n", ref.Value)); err != nil {
			return err
		}
	}
	// Now that we've populated all the remote names, we need to checkout
	// the branch.
	cmtish, err := RevParseCommitish(c, &RevParseOptions{}, org+"/"+br)
	if err != nil {
		return err
	}
	cmt, err := cmtish.CommitID(c)
	if err != nil {
		return err
	}
	// Update the master branch to point to the same commit as origin/master
	if err := UpdateRefSpec(
		c,
		UpdateRefOptions{CreateReflog: true, OldValue: CommitID{}},
		RefSpec("refs/heads/master"),
		cmt,
		"clone: "+rmt.String(),
	); err != nil {
		return err
	}

	reflog, err := c.GitDir.ReadFile("logs/refs/heads/master")
	if err != nil {
		return err
	}
	// HEAD is already pointing to refs/heads/master from init, but the
	// logs/HEAD reflog isn't created yet. We cheat by just copying the
	// one created by UpdateRefSpec above.
	if err := c.GitDir.WriteFile("logs/HEAD", reflog, 0755); err != nil {
		return err
	}

	// Finally, checkout the files. Since it's an initial clone, we just
	// do a hard reset and don't try to be intelligent about what readtree
	// does.
	return Reset(c, ResetOptions{Hard: true}, nil)
}
