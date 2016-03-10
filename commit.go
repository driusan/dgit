package main

import (
	libgit "github.com/driusan/git"
	"fmt"
)
func Commit(repo *libgit.Repository, args []string) string{


	// extract the parameters that get passed directly to commit-tree
	var messages []string
	for idx, val := range args {
		switch val {
			case "-m", "-F":
			messages = append(messages, args[idx:idx+2]...)
		}
	}
	


	// get the parent commit, if it exists
	var commitTreeArgs []string
	if parentCommit, err := getHeadId(repo); err == nil {
		commitTreeArgs = []string{"-p", parentCommit}
	}
	commitTreeArgs = append(commitTreeArgs, messages...)

	// write the current index tree and get the SHA1
	treeSha1 := WriteTree(repo)
	commitTreeArgs = append(commitTreeArgs, treeSha1)	
	commitSha1 := CommitTree(repo, commitTreeArgs)
	return commitSha1 
}
