package cmd

import (
	"flag"
	"fmt"

	"github.com/driusan/dgit/git"
)

func RevList(c *git.Client, args []string) ([]git.Sha1, error) {
	flags := flag.NewFlagSet("rev-list", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	includeObjects := flags.Bool("objects", false, "include non-commit objects in output")
	quiet := flags.Bool("quiet", false, "prevent printing of revisions")
	flags.Parse(args)
	args = flags.Args()

	excludeList := make(map[git.Sha1]struct{})
	// First get a map of excluded commitIDs
	//func revListExcludeList(c *git.Client, cmt git.CommitID, excludeList map[git.Sha1]struct{}, quiet bool) ([]git.Sha1, error) {
	for _, rev := range args {
		if rev == "" {
			continue
		}
		if rev[0] == '^' && len(rev) > 1 {
			commits, err := RevParse(c, []string{rev[1:]})
			if err != nil {
				return nil, fmt.Errorf("%s:%v", rev, err)
			}
			for _, commit := range commits {
				if err := revListExcludeList(c, git.CommitID(commit.Id), excludeList, *includeObjects); err != nil {
					return nil, err
				}
				/*
					ancestors, err := commit.Ancestors(c)
					if err != nil {
						return nil, fmt.Errorf("%s:%v", rev, err)
					}
					for _, allC := range ancestors {
						sha := git.Sha1(allC)
						if _, ok := excludeList[sha]; ok {
							continue
						}
						excludeList[sha] = (struct{}{})
						if *includeObjects {
							objs, err := allC.GetAllObjects(c)
							if err != nil {
								panic(err)
							}
							for _, o := range objs {
								excludeList[o] = struct{}{}
							}
						}

					}
				*/
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
		com, err := commits[0].CommitID(c)
		if err != nil {
			return nil, err
		}

		newobjs, err := revList(c, com, excludeList, *quiet, *includeObjects)
		if err != nil {
			return nil, err
		}
		objs = append(objs, newobjs...)
		/*
			ancestors, err := com.Ancestors(c)
			if err != nil {
				return nil, err
			}
			for _, allC := range ancestors {
				if _, ok := excludeList[git.Sha1(allC)]; !ok {
					if !*quiet {
						fmt.Printf("%v\n", git.Sha1(allC))
					}
					objs = append(objs, git.Sha1(allC))
					if *includeObjects {
						objs2, err := allC.GetAllObjects(c)
						if err != nil {
							panic(err)
						}
						for _, o := range objs2 {
							if _, okie := excludeList[o]; !okie {
								if !*quiet {
									fmt.Printf("%v\n", o.String())
								}
								objs = append(objs, git.Sha1(o))
							}
							excludeList[o] = true
						}
					}
				}
			}
		*/
	}
	return objs, nil
}

func revList(c *git.Client, cmt git.CommitID, excludeList map[git.Sha1]struct{}, quiet, includeobjs bool) ([]git.Sha1, error) {
	if _, ok := excludeList[git.Sha1(cmt)]; ok {
		return nil, nil
	}

	shas := make([]git.Sha1, 0)
	if !quiet {
		fmt.Printf("%v\n", cmt)
		if includeobjs {
			objs, err := cmt.GetAllObjects(c)
			if err != nil {
				return nil, err
			}
			for _, o := range objs {
				if _, ok := excludeList[o]; !ok {
					if !quiet {
						fmt.Printf("%v\n", o)
					}
					shas = append(shas, git.Sha1(o))
				}
				excludeList[o] = struct{}{}
			}
		}
	}
	parents, err := cmt.Parents(c)
	if err != nil {
		return nil, err
	}
	for _, p := range parents {
		if _, ok := excludeList[git.Sha1(p)]; ok {
			continue
		}
		shas = append(shas, git.Sha1(p))
		ancestors, err := revList(c, p, excludeList, quiet, includeobjs)
		if err != nil {
			return nil, err
		}
		excludeList[git.Sha1(p)] = struct{}{}
		shas = append(shas, ancestors...)
	}
	return shas, nil
}

func revListExcludeList(c *git.Client, cmt git.CommitID, excludeList map[git.Sha1]struct{}, includeobjs bool) error {
	if _, ok := excludeList[git.Sha1(cmt)]; ok {
		return nil
	}
	excludeList[git.Sha1(cmt)] = struct{}{}

	if includeobjs {
		objs, err := cmt.GetAllObjects(c)
		if err != nil {
			return err
		}
		for _, o := range objs {
			excludeList[o] = struct{}{}
		}
	}
	parents, err := cmt.Parents(c)
	if err != nil {
		return err
	}
	for _, p := range parents {
		if _, ok := excludeList[git.Sha1(p)]; ok {
			continue
		}
		if err := revListExcludeList(c, p, excludeList, includeobjs); err != nil {
			return err
		}
		excludeList[git.Sha1(p)] = struct{}{}
	}
	return nil
}
