package git

import (
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
)

// List of command line options that may be passed to RevList
type RevListOptions struct {
	Quiet, Objects bool
	MaxCount       *uint
}

var maxCountError = fmt.Errorf("Maximum number of objects has been reached")

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

// Internal pre-computed record of which symbolic references
//  are pointing to a particular ref name (without the refs/ prefix)
var symRefsForRefName map[string]string

// Internal pre-computed record of which ref names are pointing to
//  to a particular commit. Values are in the form of "HEAD -> master, branch2,
//  tag: tag1"
var refNamesForCommit map[string]string

// We have been asked to pre-compute the ref names for important
//  commits as a way to improve performance.
func buildRefNames(c *Client) error {
	symRefsForRefName = make(map[string]string)
	refNamesForCommit = make(map[string]string)

	// TODO support more symbolic refs than just HEAD
	headrefspec, err := SymbolicRefGet(c, SymbolicRefOptions{Short: true}, "HEAD")
	if err != nil {
		return err
	}
	symRefsForRefName[string(headrefspec)] = "HEAD"

	branches, err := c.GetBranches()
	if err != nil {
		return err
	}

	for _, branch := range branches {
		branchName := string(branch)

		commitId, err := RefSpec(branchName).Value(c)
		if err != nil {
			//return err There are problems dealing with hierarchical branches
			continue
		}

		branchName = branchName[len("refs/heads/"):]

		if entry, ok := symRefsForRefName[branchName]; ok {
			branchName = entry + " -> " + branchName
		}

		if entry, ok := refNamesForCommit[commitId]; ok {
			refNamesForCommit[commitId] = entry + ", " + branchName
		} else {
			refNamesForCommit[commitId] = branchName
		}
	}

	tagNames, err := TagList(c, TagOptions{}, []string{})
	if err != nil {
		return err
	}

	for _, tagName := range tagNames {
		tag := RefSpec("refs/tags/" + tagName)
		commitId, err := tag.Value(c)
		if err != nil {
			//return err There are problems dealing with hierarchical tags
			continue
		}

		if entry, ok := refNamesForCommit[commitId]; ok {
			refNamesForCommit[commitId] = entry + ", tag: " + tagName
		} else {
			refNamesForCommit[commitId] = "tag: " + tagName
		}
	}

	return nil
}

// Since libgit is somewhat out of our control and we can't implement
// a fmt.Stringer interface there, we use this helper.

// Print commit using "medium" format
func printCommitMedium(c *Client, cmt CommitID, format string) error {
	author, err := cmt.GetAuthor(c)
	if err != nil {
		return err
	}
	fmt.Printf("commit %s\n", cmt)
	if parents, err := cmt.Parents(c); len(parents) > 1 && err == nil {
		fmt.Printf("Merge: ")
		for i, p := range parents {
			fmt.Printf("%s", p)
			if i != len(parents)-1 {
				fmt.Printf(" ")
			}
		}
		fmt.Println()
	}
	date, err := cmt.GetDate(c)
	if err != nil {
		return err
	}
	fmt.Printf("Author: %v\nDate:   %v\n\n", author, date.Format("Mon Jan 2 15:04:05 2006 -0700"))
	log.Printf("Commit %v\n", cmt)

	msg, err := cmt.GetCommitMessage(c)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimSpace(msg.String()), "\n")
	for _, l := range lines {
		fmt.Printf("    %v\n", l)
	}
	fmt.Printf("\n")
	return nil
}

func printCommitOneLine(c *Client, cmt CommitID, format string) error {
	msg, err := cmt.GetCommitMessage(c)
	if err != nil {
		return err
	}
	// TODO figure out if merge parents should show up in this mode or not
	title := strings.Split(strings.TrimSpace(msg.String()), "\n")[0]
	fmt.Printf("%s %s\n", cmt, title)
	return nil
}

func printCommitFull(c *Client, cmt CommitID, format string) error {
	author, err := cmt.GetAuthor(c)
	if err != nil {
		return err
	}
	commit, err := cmt.GetCommitter(c)
	if err != nil {
		return err
	}
	fmt.Printf("commit %s\n", cmt)
	if parents, err := cmt.Parents(c); len(parents) > 1 && err == nil {
		fmt.Printf("Merge: ")
		for i, p := range parents {
			fmt.Printf("%s", p)
			if i != len(parents)-1 {
				fmt.Printf(" ")
			}
		}
		fmt.Println()
	}
	fmt.Printf("Author: %v\nCommit: %v\n\n", author, commit)

	msg, err := cmt.GetCommitMessage(c)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimSpace(msg.String()), "\n")
	for _, l := range lines {
		fmt.Printf("    %v\n", l)
	}
	fmt.Printf("\n")
	return nil
}

func printCommitFormat(c *Client, cmt CommitID, format string) error {
	output := format[7:]

	// Full commit hash
	if strings.Contains(output, "%H") {
		output = strings.Replace(output, "%H", cmt.String(), -1)
	}

	// Committer date as unix timestamp
	if strings.Contains(output, "%ct") {
		date, err := cmt.GetDate(c)
		if err != nil {
			return err
		}
		output = strings.Replace(output, "%ct", strconv.FormatInt(date.Unix(), 10), -1)
	}

	// Show the non-stylized ref names beside any relevant commit
	if strings.Contains(output, "%D") {
		output = strings.Replace(output, "%D", refNamesForCommit[cmt.String()], -1)
	}

	// TODO Add more formatting options (there are many)

	fmt.Println(output)
	return nil
}

func GetCommitPrinter(c *Client, format string) (func(*Client, CommitID, string) error, error) {
	switch format {
	case "medium":
		return printCommitMedium, nil
	case "oneline":
		return printCommitOneLine, nil
	case "full":
		return printCommitFull, nil
	}

	if strings.HasPrefix(format, "format:") && len(format) > 6 {
		if strings.Contains(format, "%d") || strings.Contains(format, "%D") {
			buildRefNames(c)
		}
		return printCommitFormat, nil
	}

	return nil, fmt.Errorf("Unsupported format: %s\n", format)
}
