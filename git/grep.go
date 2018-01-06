package git

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
)

type GrepOptions struct {
	ShowFilename      bool
	Text              bool
	TextConv          bool
	IgnoreCase        bool
	IgnoreBinary      bool
	Cached            bool
	NoIndex           bool
	Untracked         bool
	NoExcludeStandard bool
	ExcludeStandard   bool
	RecurseSubmodules bool

	ParentBaseName      string
	MaxDepth            int
	WordRegex           bool
	InvertMatch         bool
	FullName            bool
	FixedStrings        bool
	LineNumbers         bool
	NameOnly            bool
	OpenFilesInPager    string
	NullTerminate       bool
	Count               bool
	LineBreaks, Heading bool
	Invert              bool
	ExtendedRegexp      bool
	Colour              bool
	ShowFunction        bool
	FunctionContext     bool
	NumThreads          int
	Quiet               bool
	File                File

	LeadingContext, TrailingContext int

	// If non-nil, Grep will use opts.Stdout to write results
	// to instead of os.Stdout.
	Stdout io.Writer
}

func Grep(c *Client, opts GrepOptions, pattern string, tree Treeish, pathspec []File) error {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	lsopts := LsFilesOptions{Cached: true, ExcludeStandard: true}
	if opts.Untracked {
		lsopts.Others = true
	}
	if opts.NoExcludeStandard {
		lsopts.ExcludeStandard = false
	}
	files, err := LsFiles(c, lsopts, pathspec)
	if err != nil {
		return err
	}
	for _, f := range files {
		fname, err := f.PathName.FilePath(c)
		if err != nil {
			return err
		}
		if err := grepFile(fname.String(), pattern, opts); err != nil {
			return err
		}
	}
	return nil
}

func grepFile(filename, pattern string, opts GrepOptions) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	re, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(file)

	for lineNo := 1; scanner.Scan(); lineNo++ {
		line := scanner.Bytes()
		if re.Match(line) {
			if opts.LineNumbers {
				fmt.Fprintf(opts.Stdout, "%v:%v: %s\n", filename, lineNo, line)
			} else {
				fmt.Fprintf(opts.Stdout, "%v: %s\n", filename, line)
			}
		}
	}
	return nil
}
