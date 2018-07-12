package cmd

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/driusan/dgit/git"
)

func parseCacheInfo(input string) (git.CacheInfo, error) {
	pieces := strings.SplitN(input, ",", 4)
	if len(pieces) != 3 {
		return git.CacheInfo{}, fmt.Errorf("Invalid --cacheinfo format")
	}
	ret := git.CacheInfo{}
	switch pieces[0] {
	case "100644":
		ret.Mode = git.ModeBlob
	case "100755":
		ret.Mode = git.ModeExec
	case "120000":
		ret.Mode = git.ModeSymlink
		//		case "160000":
		//			ret.Mode = git.Commit
		//	An index can't contain a commit..
		//		case "040000", "40000":
		//			ret.EntryMode = git.Tree
		// an index can't contain a tree, either..
	default:
		return git.CacheInfo{}, fmt.Errorf("Invalid EntryMode")
	}

	sha1, err := git.Sha1FromString(pieces[1])
	if err != nil {
		return git.CacheInfo{}, err
	}
	ret.Sha1 = sha1
	ret.Path = git.IndexPath(pieces[2])
	return ret, nil
}

func UpdateIndex(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("update-index", flag.ExitOnError)
        flags.SetOutput(os.Stdout)
	flags.Usage = func() {
		flag.Usage()
                fmt.Printf("\n\nOptions:\n")
		flags.PrintDefaults()
	}

	opts := git.UpdateIndexOptions{}

	flags.BoolVar(&opts.Add, "add", false, "Add missing files to the index")
	flags.BoolVar(&opts.Remove, "remove", false, "Remove files that don't exist from the index")

	flags.BoolVar(&opts.Refresh, "refresh", false, "Check if merges or updates are needed by checking stat information")
	flags.BoolVar(&opts.Quiet, "q", false, "If --refresh finds the index needs an update, continue anyways")
	flags.BoolVar(&opts.IgnoreSubmodules, "ignore-submodules", false, "Do not try to update submodules")
	flags.BoolVar(&opts.Unmerged, "unmerged", false, "If --refresh finds unmerged changes in the index, do not error out")
	flags.BoolVar(&opts.IgnoreMissing, "ignore-missing", false, "Ignore missing files during --refresh")
	cacheinfo := flags.String("cacheinfo", "", "Directly set cacheinfo. Only the comma separated form of --cacheinfo mode,object,path is supported")
	flags.BoolVar(&opts.IndexInfo, "index-info", false, "Read index information from stdin")
	chmod := flags.String("chmod", "", "set the executable permissions on the updated file(s). Must be (+/-)x")
	assumeunchanged := flags.Bool("assume-unchanged", false, "Set the assume unchanged bit")
	noassumeunchanged := flags.Bool("no-assume-unchanged", false, "Unset the --assume-unchanged bit")
	flags.BoolVar(&opts.ReallyRefresh, "really-refresh", false, "Like --refresh, but ignores the assume-unchanged bit")
	skipworktree := flags.Bool("skip-worktree", false, "Set the skip worktree bit")
	noskipworktree := flags.Bool("no-skip-worktree", false, "Unset the --skip-worktree bit")
	flags.BoolVar(&opts.Again, "again", false, "Runs git-update-index itself on the paths whose index entries are different from those from the HEAD commit. I don't know what this means, but it's what the man page says.")
	g := flags.Bool("g", false, "Alias for --again")
	flags.BoolVar(&opts.Unresolve, "unresolve", false, "Restores the unmerged or needs updating state of a file during a merge")
	flags.BoolVar(&opts.InfoOnly, "info-only", false, "Do not create objects in the database, just insert their object IDs into the index")
	flags.BoolVar(&opts.ForceRemove, "force-remove", false, "Remove the file from the index even when it exists in the working directory. Implies --remove")
	flags.BoolVar(&opts.Replace, "replace", false, "When a path exists in the index, allow a file with the same name to replace it")
	stdin := flags.Bool("stdin", false, "Instead of reading paths from the command line, read them from stdin")
	flags.BoolVar(&opts.Verbose, "verbose", false, "Report what is being added and removed from the index")
	flags.IntVar(&opts.IndexVersion, "index-version", 0, "Write the resulting index using index version. Only 2 is supported.")
	flags.BoolVar(&opts.NullTerminate, "z", false, "Use nil instead of newline to terminate paths from stdin")

	splitindex := flags.Bool("split-index", false, "Use a split index. Unimplemented.")
	nosplitindex := flags.Bool("no-split-index", false, "Override --split-index or core.splitIndex setting")

	untrackedcache := flags.Bool("untracked-cache", false, "Enable untracked cache feature. Unimplemented.")
	nountrackedcache := flags.Bool("no-untracked-cache", false, "Override --untracked-cache")
	testuntrackedcache := flags.Bool("test-untracked-cache", false, "Perform test to check if untracked cache can be used")
	forceuntrackedcache := flags.Bool("force-untracked-cache", false, "Same as --untracked-cache")

	flags.Parse(args)
	if *stdin {
		opts.Stdin = os.Stdin
	}
	if *g {
		opts.Again = true
	}

	if *splitindex {
		opts.SplitIndex.Modify = true
		opts.SplitIndex.Value = true
	}
	if *nosplitindex {
		opts.SplitIndex.Modify = true
		opts.SplitIndex.Value = false
	}

	switch *chmod {
	case "":
		opts.Chmod.Modify = false
	case "+x":
		opts.Chmod.Modify = true
		opts.Chmod.Value = true
	case "-x":
		opts.Chmod.Modify = true
		opts.Chmod.Value = false
	default:
		return fmt.Errorf("Invalid value for --chmod option. Must be +x or -x")
	}

	if *untrackedcache || *forceuntrackedcache {
		opts.UntrackedCache.Modify = true
		opts.UntrackedCache.Value = true
	}

	if *nountrackedcache {
		opts.UntrackedCache.Modify = true
		opts.UntrackedCache.Value = false
	}
	if *testuntrackedcache {
		return fmt.Errorf("UntrackedCache not implemented.")
	}

	if *assumeunchanged {
		opts.AssumeUnchanged.Modify = true
		opts.AssumeUnchanged.Value = true
	}
	if *noassumeunchanged {
		opts.AssumeUnchanged.Modify = true
		opts.AssumeUnchanged.Value = false
	}

	if *skipworktree {
		opts.SkipWorktree.Modify = true
		opts.SkipWorktree.Value = true
	}
	if *noskipworktree {
		opts.SkipWorktree.Modify = true
		opts.SkipWorktree.Value = false
	}

	if *cacheinfo != "" {
		ci, err := parseCacheInfo(*cacheinfo)
		if err != nil {
			return err
		}
		opts.CacheInfo = ci
	}

	vals := flags.Args()
	files := make([]git.File, len(vals), len(vals))
	for i, val := range vals {
		files[i] = git.File(val)
	}

	// Load the index file and call UpdateIndex on it.
	idx, err := c.GitDir.ReadIndex()
	if err != nil {
		return err
	}
	newidx, err := git.UpdateIndex(c, idx, opts, files)

	if err != nil {
		return err
	}

	// Write the index file back to disk if there were no errors.
	f, err := c.GitDir.Create(git.File("index"))
	if err != nil {
		return err
	}
	defer f.Close()
	return newidx.WriteIndex(f)
}
