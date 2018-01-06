package cmd

import (
	"flag"
	"fmt"

	"github.com/driusan/dgit/git"
)

func Grep(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("grep", flag.ExitOnError)
	flags.Usage = func() {
		flag.Usage()
		flags.PrintDefaults()
	}

	opts := git.GrepOptions{
		ShowFilename: true,
	}

	flags.BoolVar(&opts.Text, "text", false, "Process binary files as if they were text")
	a := flags.Bool("a", false, "Alias of --text")
	flags.BoolVar(&opts.TextConv, "textconv", false, "Honour textconv filter settings")
	flags.BoolVar(&opts.IgnoreCase, "ignore-case", false, "Ignore case differences between the patterns and the files")
	i := flags.Bool("i", false, "Alias of --ignore-case")

	flags.BoolVar(&opts.IgnoreBinary, "I", false, "Ignore binary files")

	flags.BoolVar(&opts.Cached, "cached", false, "Instead of searching tracked files, search blobs in the index")
	flags.BoolVar(&opts.NoIndex, "no-index", false, "Search files in the current directory that are not managed by git")
	flags.BoolVar(&opts.Untracked, "untracked", false, "Search both tracked and untracked files")
	flags.BoolVar(&opts.NoExcludeStandard, "no-exclude-standard", false, "Do not honour .gitignore")
	flags.BoolVar(&opts.ExcludeStandard, "exclude-standard", false, "Do not pay attention to files specified via .gitignore")
	flags.BoolVar(&opts.RecurseSubmodules, "RecurseSubmodules", false, "Recurse into submodules")
	flags.StringVar(&opts.ParentBaseName, "parent-basename", "", "Unused")
	flags.IntVar(&opts.MaxDepth, "max-depth", -1, "Descend at most maxdepths levels of directories")

	flags.BoolVar(&opts.WordRegex, "word-regexp", false, "Match only at word boundaries")
	w := flags.Bool("w", false, "Alias of --word-regexp")

	flags.BoolVar(&opts.InvertMatch, "invert-match", false, "Select non-matching lines")
	v := flags.Bool("v", false, "Alias of ----invert-match")

	h := flags.Bool("h", false, "Do not show filenames for matches")
	H := flags.Bool("H", false, "Negate a previous -h.")

	flags.BoolVar(&opts.FullName, "full-name", false, "Show paths relative to top level directory, not current directory")

	extended := flags.Bool("extended-regexp", false, "Use POSIX extended regexps")
	E := flags.Bool("E", false, "Alias of --extended-regexp")

	basicregexp := flags.Bool("basic-regexp", true, "Use basic regexps (default)")
	G := flags.Bool("G", false, "Alias of --basic-regexp")

	_ = flags.Bool("perl-regexp", false, "Use perl-compatible regular expressions. (Unused, only parsed for compatibility with git)")
	_ = flags.Bool("P", false, "Alias of --perl-regexp")

	flags.BoolVar(&opts.FixedStrings, "fixed-strings", false, "Use fixed strings as patterns, not regexps")
	F := flags.Bool("F", false, "Alias of --fixed-strings")

	flags.BoolVar(&opts.LineNumbers, "line-number", false, "Show line numbers for lines")
	n := flags.Bool("n", false, "Alias of --line-number")

	flags.BoolVar(&opts.NameOnly, "name-only", false, "Only show the file names of matches, not every matched lines")
	fileswithmatches := flags.Bool("files-with-matches", false, "Alias of --name-only")
	l := flags.Bool("l", false, "Alias of --name-only")

	fileswithoutmatches := flags.Bool("files-without-matches", false, "Only show files that do not have matches")
	L := flags.Bool("L", false, "Alias of --files-without-matches")

	flags.StringVar(&opts.OpenFilesInPager, "open-files-in-pager", "", "Open matching files (not the output of grep in pager")
	O := flags.String("O", "", "Alias of --open-files-in-pager")

	flags.BoolVar(&opts.NullTerminate, "null", false, "Use nil to separate file names")
	z := flags.Bool("z", false, "Alias of --null")

	flags.BoolVar(&opts.Count, "count", false, "Instead of showing lines that match, show the number of lines")
	count := flags.Bool("c", false, "Alias of --count")

	colour := flags.String("color", "", "Highlight matches in colour")
	nocolour := flags.Bool("no-color", false, "Turn off match highlighting")

	flags.BoolVar(&opts.LineBreaks, "break", false, "Print an empty line between matches in different files")
	flags.BoolVar(&opts.Heading, "heading", false, "Show the filename above the matches instead of at the start of each line")

	flags.BoolVar(&opts.ShowFunction, "show-function", false, "Show the preceding line that contains the function name")
	p := flags.Bool("p", false, "Alias of --show-function")

	context := flags.Int("context", 0, "Show n leading and trailing lines")
	C := flags.Int("C", 0, "Alias of --context")

	aftercontext := flags.Int("after-context", 0, "Show n lines of trailing context")
	A := flags.Int("A", 0, "Alias of --after-context")
	beforecontext := flags.Int("before-context", 0, "Show n lines of leading context")
	B := flags.Int("B", 0, "Alias of --before-context")

	flags.BoolVar(&opts.FunctionContext, "function-context", false, "Show the entire function for context")
	W := flags.Bool("W", false, "Alias of --function-context")

	flags.IntVar(&opts.NumThreads, "threads", 0, "Spawn n worker threads for grep")

	f := flags.String("f", "", "Read patterns from file, one per line")
	flags.BoolVar(&opts.Quiet, "quiet", false, "Do not output matched lines")
	e := flags.String("e", "", "The next parameter is the pattern. (Used if pattern starts with -)")
	// FIXME: Missing:
	// --and, --or, --not
	// -- all-match
	flags.Parse(args)
	args = flags.Args()

	if *W {
		opts.FunctionContext = true
	}
	if *f != "" {
		opts.File = git.File(*f)
	}
	if *p {
		opts.ShowFunction = true
	}

	if *context > 0 {
		opts.LeadingContext = *context
		opts.TrailingContext = *context
	} else if *C > 0 {
		opts.LeadingContext = *C
		opts.TrailingContext = *C
	}

	if *aftercontext > 0 {
		opts.TrailingContext = *aftercontext
	} else if *A > 0 {
		opts.TrailingContext = *aftercontext
	}
	if *beforecontext > 0 {
		opts.LeadingContext = *beforecontext
	} else if *B > 0 {
		opts.LeadingContext = *B
	}
	if *count {
		opts.Count = true
	}
	if *w {
		opts.WordRegex = true
	}
	if *v {
		opts.Invert = true
	}
	if *z {
		opts.NullTerminate = true
	}

	if *O != "" {
		opts.OpenFilesInPager = *O
	}
	if *fileswithmatches || *l {
		opts.NameOnly = true
	}
	if *fileswithoutmatches || *L {
		opts.NameOnly = true
		opts.Invert = true
	}

	if *extended || *E {
		opts.ExtendedRegexp = true
	}
	if *basicregexp || *G {
		opts.ExtendedRegexp = false
	}
	if *F {
		opts.FixedStrings = false
	}

	if *n {
		opts.LineNumbers = true
	}
	if *a {
		opts.Text = true
	}
	if *i {
		opts.IgnoreCase = true
	}

	if *h {
		opts.ShowFilename = true
	}
	if *H {
		opts.ShowFilename = true
	}

	switch *colour {
	case "", "never":
		opts.Colour = false
	case "always":
		opts.Colour = true
	default:
		return fmt.Errorf("Invalid valid value for colour: %v", *colour)
	}
	if *nocolour {
		opts.Colour = false
	}

	var pattern string
	if *e != "" {
		pattern = *e
	} else if len(args) > 0 {
		pattern = args[0]
		args = args[1:]
	}
	if pattern == "" {
		return fmt.Errorf("No pattern given.")
	}

	var tree git.Treeish
	if len(args) > 0 {
		if t, err := git.RevParseTreeish(c, &git.RevParseOptions{}, args[0]); err == nil {
			tree = t
			args = args[1:]
		}
	}
	var paths []git.File
	for _, f := range args {
		paths = append(paths, git.File(f))
	}
	return git.Grep(c, opts, pattern, tree, paths)
}
