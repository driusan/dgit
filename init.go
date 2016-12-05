package main

import (
	"io/ioutil"
	"os"
)

func Init(c *Client, args []string) {
	if len(args) > 0 {
		if dir := args[len(args)-1]; dir != "init" {
			err := os.MkdirAll(dir, 0755)
			if err != nil {
				panic("Couldn't create directory for initializing git.")
			}
			err = os.Chdir(dir)
			if err != nil {
				panic("Couldn't change working directory while initializing git.")
			}
			if c != nil {
				c.GitDir = GitDir(dir + ".git/")
			}
		}
	}
	// These are all the directories created by a clean "git init"
	// with the canonical git implementation
	os.Mkdir(".git", 0755)
	os.MkdirAll(".git/objects/pack", 0755)
	os.MkdirAll(".git/objects/info", 0755)
	os.MkdirAll(".git/info", 0755)  // Should have exclude file in it
	os.MkdirAll(".git/hooks", 0755) // should have sample hooks in it.
	os.MkdirAll(".git/branches", 0755)
	os.MkdirAll(".git/refs/heads", 0755)
	os.MkdirAll(".git/refs/tags", 0755)

	ioutil.WriteFile(".git/HEAD", []byte("ref: refs/heads/master\n"), 0644)
	ioutil.WriteFile(".git/config", []byte("[core]\n\trepositoryformatversion = 0\n\tbare = false\n"), 0644)
	ioutil.WriteFile(".git/description", []byte("Unnamed repository; edit this file 'description' to name the repository.\n"), 0644)

}
