package git

import (
	"fmt"
	"io"
	"os"
)

// List of command line options that may be passed to RevList
type RevListOptions struct {
	Quiet, Objects bool
	MaxCount       *uint
	VerifyObjects  bool
	All            bool
}

var maxCountError = fmt.Errorf("Maximum number of objects has been reached")

func RevList(c *Client, opt RevListOptions, w io.Writer, includes, excludes []Commitish) ([]Sha1, error) {
	var vals []Sha1
	err := RevListCallback(c, opt, includes, excludes, func(s Sha1) error {
		vals = append(vals, s)
		if !opt.Quiet {
			fmt.Fprintf(w, "%v\n", s)
		}
		if opt.VerifyObjects {
			switch t := s.Type(c); t {
			case "commit":
				if err := verifyCommit(c, FsckOptions{}, CommitID(s)); err != nil {
					fmt.Fprintln(os.Stderr, err)
					return err
				}
			case "tree":
				if err := verifyTree(c, FsckOptions{}, TreeID(s)); err != nil {
					fmt.Fprintln(os.Stderr, err)
					return err
				}
			case "tag":
				if err := verifyTag(c, FsckOptions{}, s); err != nil {
					fmt.Fprintln(os.Stderr, err)
					return err[0]
				}
			case "blob":
				if err := verifyBlob(c, FsckOptions{}, os.Stderr, s); err != nil {
					return err
				}
			default:
				return fmt.Errorf("Invalid object type %v", t)
			}
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

	callbackCount := uint(0)
	callbackCountWrapper := func(s Sha1) error {
		callbackCount++
		if opt.MaxCount != nil && callbackCount > *opt.MaxCount {
			return maxCountError
		}

		return callback(s)
	}

	err := revListCallback(c, opt, cIDs, excludeList, callbackCountWrapper)
	if err == maxCountError {
		return nil
	}
	return err
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
