package git

import (
	"fmt"
	"io"
)

// List of command line options that may be passed to RevList
type RevListOptions struct {
	Quiet, Objects bool
}

func RevList(c *Client, opt RevListOptions, w io.Writer, includes, excludes []Commitish) ([]Sha1, error) {
	var vals []Sha1
	err := RevListCallback(c, opt, includes, excludes, func(s Sha1) error {
		vals = append(vals, s)
		if !opt.Quiet {
			fmt.Fprintf(w, "%v\n", s)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return vals, nil
}

func RevListCallback(c *Client, opt RevListOptions, includes, excludes []Commitish, callback func(Sha1) error) error {
	excludeList := make(map[Sha1]struct{})
	buildExcludeList := func(s Sha1) error {
		if _, ok := excludeList[s]; ok {
			return nil
		}
		if opt.Objects {
			cmt := CommitID(s)
			_, err := cmt.GetAllObjectsExcept(c, excludeList)
			if err != nil {
				return err
			}
		}
		excludeList[s] = struct{}{}
		return nil
	}

	if len(excludes) > 0 {
		var cIDs []CommitID = make([]CommitID, 0, len(excludes))
		for _, e := range excludes {
			cmt, err := e.CommitID(c)
			if err != nil {
				return err
			}
			cIDs = append(cIDs, cmt)
		}
		if err := revListCallback(c, opt, cIDs, excludeList, buildExcludeList); err != nil {
			return err
		}
	}

	cIDs := make([]CommitID, 0, len(includes))
	for _, i := range includes {
		cmt, err := i.CommitID(c)
		if err != nil {
			return err
		}
		cIDs = append(cIDs, cmt)
	}
	return revListCallback(c, opt, cIDs, excludeList, callback)
}

func revListCallback(c *Client, opt RevListOptions, commits []CommitID, excludeList map[Sha1]struct{}, callback func(Sha1) error) error {
	for _, cmt := range commits {
		if _, ok := excludeList[Sha1(cmt)]; ok {
			continue
		}

		if err := callback(Sha1(cmt)); err != nil {
			return err
		}
		excludeList[Sha1(cmt)] = struct{}{}

		if opt.Objects {
			objs, err := cmt.GetAllObjectsExcept(c, excludeList)
			if err != nil {
				return err
			}
			for _, o := range objs {
				if err := callback(o); err != nil {
					return err
				}
			}
		}
		parents, err := cmt.Parents(c)
		if err != nil {
			return err
		}
		if err := revListCallback(c, opt, parents, excludeList, callback); err != nil {
			return err
		}
	}
	return nil
}
