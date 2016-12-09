package main

import (
	"fmt"
	libgit "github.com/driusan/git"
	"io/ioutil"
	"os"
)

func Reset(c *Client, repo *libgit.Repository, args []string) {
	commitId, err := c.GetHeadID()
	var resetPaths = false
	var mode string = "mixed"
	if err != nil {
		fmt.Fprintf(os.Stderr, "Couldn't find HEAD commit.\n")
	}
	for _, val := range args {
		if _, err := os.Stat(val); err == nil {
			resetPaths = true
			panic("TODO: I'm not prepared to handle git reset <paths>")
		}
		// The better way to do this would have been:
		// git reset [treeish] <paths>:
		//  stat val
		//      if valid file:
		//          reset index to status at [treeish]
		// (opposite of git add)
		//

		// Expand the parameter passed to a CommitID. We need
		// the CommitID that it refers to no matter what mode
		// we're in, but if we've already found a path already
		// then the time for a treeish option is past.
		if val[0] != '-' && resetPaths == false {
			commitId = getTreeishId(c, repo, val)
		} else {
			switch val {
			case "--soft":
				mode = "soft"
			case "--mixed":
				mode = "mixed"
			case "--hard":
				mode = "hard"
			default:
				fmt.Fprintf(os.Stderr, "Unknown option: %s", val)
			}
		}
	}
	if resetPaths == false {
		// no paths were found. This is the form
		//  git reset [mode] commit
		// First, update the head reference for all modes
		branchName := c.GetHeadBranch()
		err := ioutil.WriteFile(c.GitDir.String()+"/refs/heads/"+branchName,
			[]byte(fmt.Sprintf("%s", commitId)),
			0644,
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error updating head reference: %s\n", err)
			return
		}

		// mode: soft: do not touch working tree or index
		//       mixed (default): reset the index but not working tree
		//       hard: reset the index and the working tree
		switch mode {
		case "soft":
			// don't do anything for soft reset other than update
			// the head reference
		case "hard":
			resetIndexFromCommit(c, commitId)
			err := c.ResetWorkTree()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error updating head reference: %s\n", err)

			}
		case "mixed":
			fallthrough
		default:
			resetIndexFromCommit(c, commitId)
		}

	}
}
