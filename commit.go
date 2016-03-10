package main

import (
	libgit "github.com/driusan/git"
)

func Commit(repo *libgit.Repository, args []string) string {
	// get the parent commit, if it exists
	var commitTreeArgs []string
	if parentCommit, err := getHeadId(repo); err == nil {
		commitTreeArgs = []string{"-p", parentCommit}
	}

	// extract the message parameters that get passed directly
	//to commit-tree
	var messages []string
	for idx, val := range args {
		switch val {
		case "-m", "-F":
			messages = append(messages, args[idx:idx+2]...)
		}
	}
	commitTreeArgs = append(commitTreeArgs, messages...)

	// write the current index tree and get the SHA1
	treeSha1 := WriteTree(repo)
	commitTreeArgs = append(commitTreeArgs, treeSha1)

	// write the commit tree
	commitSha1 := CommitTree(repo, commitTreeArgs)

	UpdateRef(repo, []string{"-m", "commit from go-git", "HEAD", commitSha1})
	return commitSha1
}
