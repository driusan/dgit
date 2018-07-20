package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/driusan/dgit/git"
)

func Apply(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("apply", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	opts := git.ApplyOptions{}

	flags.BoolVar(&opts.Stat, "stat", false, "Instead of applying the patch, output diffstat for the input.")
	flags.BoolVar(&opts.NumStat, "num-stat", false, "Similar to --stat, but shows added and deleted lines in decimal notation")
	flags.BoolVar(&opts.Summary, "summary", false, "Instead of applying the patch, output a condensed summary of information obtained from diff headers")
	flags.BoolVar(&opts.Check, "check", false, "Instead of applying the patch, see if it applies cleanly")
	flags.BoolVar(&opts.Index, "index", false, "When checking or applying the patch, apply it to the index too")
	flags.BoolVar(&opts.Cached, "cached", false, "Apply the patch to the index without touching the working tree")
	flags.BoolVar(&opts.ThreeWay, "3way", false, "When the patch does not apply cleanly, fall back on a 3-way merge with conflict markers")
	flags.BoolVar(&opts.ThreeWay, "3", false, "Alias of --3way")
	flags.StringVar(&opts.BuildFakeAncestor, "build-fake-ancestor", "", "I have no idea what this means")
	flags.BoolVar(&opts.Reverse, "reverse", false, "Apply the patch in reverse")
	flags.BoolVar(&opts.Reverse, "R", false, "Apply the patch in reverse")
	flags.BoolVar(&opts.Reject, "reject", false, "Instead of atomically applying the patch, leave the rejected hunks in .rej files")

	flags.BoolVar(&opts.NullTerminate, "z", false, "Null terminate paths with --num-stat")

	flags.IntVar(&opts.Strip, "p", 1, "Remove n leading slashes from diff paths")
	flags.IntVar(&opts.Context, "C", -1, "Ensure at least <n> lines of surrounding context match before and after each change.")

	flags.BoolVar(&opts.UnidiffZero, "unidiff-zero", false, "Allow unified diff with no context lines")
	flags.BoolVar(&opts.ForceApply, "apply", false, "Apply patch even when using an option that disables apply")

	flags.BoolVar(&opts.NoAdd, "no-add", false, "When applying a patch, ignore additions made by the patch")

	_ = flags.Bool("allow-binary-replacement", true, "No-op, for compatibility with git, which only has it for historical compatibility")
	_ = flags.Bool("binary", true, "Alias of --binary")

	flags.StringVar(&opts.ExcludePattern, "exclude", "", "Don't apply changes to files matching the given pattern")
	flags.StringVar(&opts.IncludePattern, "include", "", "Only apply to files matching the given pattern")

	flags.BoolVar(&opts.InaccurateEof, "inaccurate-eof", false, "Apply patches from diffs with inaccurate EOFs")
	whitespace := flags.String("whitespace", "warn", "Determine how to handle patches with whitespace errors")

	flags.BoolVar(&opts.Verbose, "verbose", false, "Report progress to stderr")
	flags.BoolVar(&opts.Verbose, "v", false, "Alias of --verbose")

	flags.BoolVar(&opts.Recount, "recount", false, "Do not trust the line counts from the patch")

	flags.StringVar(&opts.Directory, "directory", "", "Prepend directory to all filenames")

	flags.BoolVar(&opts.UnsafePaths, "unsafe-paths", false, "Allow patching of files outside the work tree")

	flags.Parse(args)
	args = flags.Args()

	switch *whitespace {
	case "nowarn", "warn", "fix", "error", "error-all":
		opts.Whitespace = *whitespace
	default:
		return fmt.Errorf("Invalid option for --whitespace")
	}
	if opts.ThreeWay {
		opts.Index = false
		if opts.Reject || opts.Cached {
			fmt.Fprintf(flag.CommandLine.Output(), "--3way is incompatible with --reject and --cached\n")
			flags.Usage()
			os.Exit(2)
		}
	}

	var patches []git.File
	for _, f := range args {
		patches = append(patches, git.File(f))
	}
	return git.Apply(c, opts, patches)
}
