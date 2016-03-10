package main

import (
	"bytes"
	"fmt"
	libgit "github.com/driusan/git"
	"os"
	"time"
)

func getAuthor(repo *libgit.Repository) string {
	configFile, err := os.Open(os.Getenv("HOME") + "/.gitconfig")
	config := parseConfig(repo, configFile)
	if err != nil {
		panic(err)
	}

	name := config.GetConfig("user.name")
	email := config.GetConfig("user.email")
	return fmt.Sprintf("%s <%s>", name, email)

}
func CommitTree(repo *libgit.Repository, args []string) string {
	content := bytes.NewBuffer(nil)

	var parents []string
	var messageString, messageFile string
	var skipNext bool
	var tree string
	for idx, val := range args {
		if idx == 0 && val[0] != '-' {
			tree = val
			continue
		}

		if skipNext == true {
			skipNext = false
			continue
		}
		switch val {
		case "-p":
			parents = append(parents, args[idx+1])
			skipNext = true
		case "-m":
			messageString += "\n" + args[idx+1] + "\n"
			skipNext = true
		case "-F":
			messageFile = args[idx+1]
			skipNext = true

		}
	}
	if messageString == "" {
		if messageFile != "" {
			// TODO: READ commit message from messageFile here
		} else {
			// If neither messageString nor messageFile are set, read
			// from STDIN
		}
		panic("Must provide message with -m parameter to commit-tree")
	}
	if tree == "" {
		tree = args[len(args)-1]
	}
	// TODO: Validate tree id
	fmt.Fprintf(content, "tree %s\n", tree)
	for _, val := range parents {
		fmt.Fprintf(content, "parent %s\n", val)
	}

	author := getAuthor(repo)
	t := time.Now()
	_, tzoff := t.Zone()
	// for some reason t.Zone() returns the timezone offset in seconds
	// instead of hours, so convert it to an hour format string
	tzStr := fmt.Sprintf("%+03d00", tzoff/(60*60))
	fmt.Fprintf(content, "author %s %d %s\n", author, t.Unix(), tzStr)
	fmt.Fprintf(content, "commiter %s %d %s\n", author, t.Unix(), tzStr)
	fmt.Fprintf(content, "%s", messageString)
	fmt.Printf("%s", content.Bytes())
	sha1, err := repo.StoreObjectLoose(libgit.ObjectCommit, bytes.NewReader(content.Bytes()))
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s", sha1)
}
