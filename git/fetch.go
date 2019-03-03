package git

import (
	"fmt"
)

type FetchOptions struct {
	FetchPackOptions

	Force bool
}

// Fetch implements the "git fetch" command, fetching  refs from rmt.
// If refs is nil, all remote refs will be fetched from the remote.
func Fetch(c *Client, opts FetchOptions, rmt Remote, refs []RefSpec) error {
	opts.FetchPackOptions.All = (refs == nil)
	opts.FetchPackOptions.Verbose = true

	var wants []Refname
	for _, ref := range refs {
		wants = append(wants, ref.Src())
	}
	newrefs, err := FetchPack(c, opts.FetchPackOptions, rmt, wants)
	if err != nil {
		if err.Error() == "Already up to date." {
			return nil
		}
		return err
	}
	if refs == nil {
		// Fake a refspec if one wasn't specified so that things to
		// to the default location under refs.
		refs = append(
			refs,
			RefSpec(fmt.Sprintf("refs/heads/*:refs/remotes/%s/*", rmt)),
		)
	}
	if c.GitDir != "" {
		for _, ref := range newrefs {
			for _, spec := range refs {
				if match, dst := ref.MatchesRefSpecSrc(spec); match {
					if !dst.Exists(c) {
						fmt.Printf("[new branch] %v", dst)
					} else {
						fmt.Printf("%v %v", ref.Value, dst)
					}
					err := UpdateRef(
						c,
						UpdateRefOptions{NoDeref: true},
						string(dst),
						CommitID(ref.Value),
						"Ref updated by fetch",
					)
					if err != nil {
						// FIXME: I don't think we should be
						// erroring out here.
						return err
					}
				}
			}
		}
	}
	return nil
}
