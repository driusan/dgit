package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/driusan/go-git/git"
)

func RevList(c *git.Client, args []string) ([]git.Sha1, error) {
	includeObjects := flag.Bool("objects", false, "include non-commit objects in output")
	quiet := flag.Bool("quiet", false, "prevent printing of revisions")
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
			commits, err := RevParse(c, []string{rev[1:]})
			if err != nil {
				panic(rev + ":" + err.Error())
			}
			for _, commit := range commits {
				ancestors := commit.Ancestors(c)
				for _, allC := range ancestors {
					excludeList[git.Sha1(allC).String()] = true
					if *includeObjects {
						objs, err := allC.GetAllObjects(c)
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
	objs := make([]git.Sha1, 0)
	// Then follow the parents of the non-excluded ones until they hit
	// something that was excluded.
	for _, rev := range args {
		if rev == "" {
			continue
		}
		if rev[0] == '^' && len(rev) > 1 {
			continue
		}
		commits, err := RevParse(c, []string{rev})
		if err != nil {
			panic(err)
		}
		com := commits[0]
		ancestors := com.Ancestors(c)
		for _, allC := range ancestors {
			if _, ok := excludeList[git.Sha1(allC).String()]; !ok {
				if !*quiet {
					fmt.Printf("%v\n", git.Sha1(allC).String())
				}
				objs = append(objs, git.Sha1(allC))
				if *includeObjects {
					objs2, err := allC.GetAllObjects(c)
					if err != nil {
						panic(err)
					}
					for _, o := range objs2 {
						if _, okie := excludeList[o.String()]; !okie {
							if !*quiet {
								fmt.Printf("%v\n", o.String())
							}
							objs = append(objs, git.Sha1(o))
						}
						excludeList[o.String()] = true
					}
				}
			}
		}
	}
	return objs, nil
}
