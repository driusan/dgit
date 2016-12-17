package main

import (
	"fmt"
	"os"
	"strings"
)

type ParsedRevision struct {
	Id       CommitID
	Excluded bool
}

func (pr ParsedRevision) IsAncestor(c *Client, child ParsedRevision) bool {
	return pr.Id.IsAncestor(c, child.Id)
}
func RevParse(c *Client, args []string) (commits []ParsedRevision, err2 error) {
	for _, arg := range args {
		switch arg {
		case "--git-dir":
			wd, err := os.Getwd()
			if err == nil {
				fmt.Printf("%s\n", strings.TrimPrefix(c.GitDir.String(), wd+"/"))
			} else {
				fmt.Printf("%s\n", c.GitDir)
			}
		default:
			if len(arg) > 0 && arg[0] == '-' {
				fmt.Printf("%s\n", arg)
			} else {
				var sha string
				var exclude bool
				var err error
				if arg[0] == '^' {
					sha = arg[1:]
					exclude = true
				} else {
					sha = arg
					exclude = false
				}

				if len(sha) == 40 {
					comm, err := Sha1FromString(sha)
					if err != nil {
						panic(err)
					}
					commits = append(commits, ParsedRevision{CommitID(comm), exclude})
					continue
				}

				if r := getSymbolicRef(c, sha); r != "" {
					comm, err := c.GetSymbolicRefCommit(r)
					if err != nil {
						err2 = err
					} else {
						commits = append(commits, ParsedRevision{CommitID(comm), exclude})
					}
					continue
				}

				comm, err := c.GetBranchCommit(sha)
				if err != nil {
					err2 = err
				} else {
					commits = append(commits, ParsedRevision{CommitID(comm), exclude})
				}
			}

		}

	}
	return
}
