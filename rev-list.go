package main

import (
	"flag"
	"fmt"
	libgit "github.com/driusan/git"
	"os"
	//"strings"
)

func RevList(repo *libgit.Repository, args []string) {
	includeObjects := flag.Bool("objects", false, "include non-commit objects in output")
	os.Args = append([]string{"git rev-list"}, args...)
	flag.Parse()
	args = flag.Args()

	excludeList := make(map[string]bool)
	// First get a map of excluded commitIDs
	for _, rev := range args {
		if rev == "" {
			continue
		}
		if rev[0] == '^' && len(rev) > 1 {
			commits, err := RevParse(repo, []string{rev[1:]})
			if err != nil {
				panic(err)
			}
			for _, commit := range commits {
				ancestors := commit.Id.Ancestors(repo)
				for _, allC := range ancestors {
					excludeList[Sha1(allC).String()] = true
					if *includeObjects {
						objs, err := allC.GetAllObjects(repo)
						if err != nil {
							panic(err)
						}
						for _, o := range objs {
							excludeList[o.String()] = true
						}
					}

				}
			}
		}
	}
	// Then follow the parents of the non-excluded ones until they hit
	// something that was excluded.
	for _, rev := range args {
		if rev == "" {
			continue
		}
		if rev[0] == '^' && len(rev) > 1 {
			continue
		}
		commits, err := RevParse(repo, []string{rev})
		if err != nil {
			panic(err)
		}
		com := commits[0]
		ancestors := com.Id.Ancestors(repo)
		for _, allC := range ancestors {
			if _, ok := excludeList[Sha1(allC).String()]; !ok {
				fmt.Printf("%v\n", Sha1(allC).String())
				if *includeObjects {
					objs, err := allC.GetAllObjects(repo)
					if err != nil {
						panic(err)
					}
					for _, o := range objs {
						if _, okie := excludeList[o.String()]; !okie {
							fmt.Printf("%v\n", o.String())
						}
						excludeList[o.String()] = true
					}
				}
			}
		}

		//lgCommits := repo.CommitsBefore(com.String())

	}
}
