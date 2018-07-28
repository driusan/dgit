package cmd

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/driusan/dgit/git"
)

func CheckIgnore(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("check-ignore", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	quiet := false
	flags.BoolVar(&quiet, "quiet", false, "Don't output anything, just set exit status. This is only valid with a single pathname.")
	flags.BoolVar(&quiet, "q", false, "Alias for --quiet")
	verbose := false
	flags.BoolVar(&verbose, "verbose", false, "Also output details about the matching pattern (if any) for each given pathname.")
	flags.BoolVar(&verbose, "v", false, "Alias for --verbose")
	nonMatch := false
	flags.BoolVar(&nonMatch, "non-matching", false, "Show given paths which don’t match any pattern.")
	flags.BoolVar(&nonMatch, "n", false, "Alias for --non-matching.")

	stdin := false
	flags.BoolVar(&stdin, "stdin", false, "Read pathnames from the standard input, one per line, instead of from the command-line.")

	noIndex := false
	flags.BoolVar(&noIndex, "no-index", false, "Don’t look in the index when undertaking the checks.")

	machine := false
	flags.BoolVar(&machine, "z", false, "The output format is modified to be machine-parseable.")

	flags.Parse(args)
	args = flags.Args()

	if dir, err := os.Getwd(); err == nil && (dir == c.GitDir.String() || strings.HasPrefix(dir, c.GitDir.String()+"/")) {
		fmt.Fprintf(flag.CommandLine.Output(), "fatal: This operation must be run in a work tree\n")
		flags.Usage()
		os.Exit(128)
	}

	if machine && !stdin {
		fmt.Fprintf(flag.CommandLine.Output(), "fatal: -z only makes sense with --stdin\n")
		flags.Usage()
		os.Exit(128)
	}

	if !stdin && len(args) < 1 {
		fmt.Fprintf(flag.CommandLine.Output(), "fatal: no path specified\n")
		flags.Usage()
		os.Exit(128)
	} else if stdin && len(args) > 0 {
		fmt.Fprintf(flag.CommandLine.Output(), "fatal: cannot specify pathnames with --stdin\n")
		flags.Usage()
		os.Exit(128)
	} else if quiet && len(args) != 1 && !stdin {
		fmt.Fprintf(flag.CommandLine.Output(), "fatal: --quiet is only valid with a single pathname\n")
		flags.Usage()
		os.Exit(128)
	} else if quiet && verbose {
		fmt.Fprintf(flag.CommandLine.Output(), "fatal: cannot have both --quiet and --verbose\n")
		flags.Usage()
		os.Exit(128)
	}

	if !stdin {
		paths := make([]git.File, 0, len(args))

		// Assemble the list of files, checking for any invalid input
		for _, p := range args {
			f := git.File(p)
			if s, submodule, _ := f.IsInSubmodule(c); s {
				fmt.Fprintf(os.Stderr, "fatal: Pathspec '%v' is in submodule '%s'\n", p, submodule)
				os.Exit(128)
			}
			if i, _ := f.IsInsideSymlink(); i {
				fmt.Fprintf(os.Stderr, "fatal: pathspec '%v' is beyond a symbolic link\n", p)
				os.Exit(128)
			}

			paths = append(paths, git.File(p))
		}

		// Invoke the ignore routines directly, rather than relying on
		//  LsFiles because we need to be able to do things such as
		//  return non-matches and match details that aren't supported there.
		// Note that check-ignore has no choice but to use standard ignore
		//  patterns. There is no way to specify custom patterns on the command-line.
		standardIgnores, err := git.StandardIgnorePatterns(c, paths)
		if err != nil {
			return err
		}

		exitCode := 1

		for _, path := range paths {
			matches, err := git.MatchIgnores(c, standardIgnores, []git.File{path})
			if err != nil {
				return err
			}

			match := matches[0]

			// Zero out any match pattern if it's in the index and the no-index options was
			//  not specified.
			if !noIndex {
				entries, _ := git.LsFiles(c, git.LsFilesOptions{Cached: true, Others: false, ExcludeStandard: false}, []git.File{path})
				if len(entries) > 0 {
					match.IgnorePattern = git.IgnorePattern{}
				}
			}

			if match.Pattern != "" || nonMatch {
				if !quiet && !verbose {
					fmt.Printf("%s\n", match.PathName)
				} else if !quiet && verbose {
					fmt.Printf("%s\n", match)
				}

				if match.Pattern != "" {
					exitCode = 0
				}
			}
		}

		os.Exit(exitCode)
	} else {
		reader := bufio.NewReader(os.Stdin)
		exitCode := 1

		for {
			path := ""
			isEof := false

			if !machine {
				for {
					lineBytes, isPrefix, err := reader.ReadLine()
					path = path + string(lineBytes)
					if err == io.EOF {
						isEof = true
					}
					if !isPrefix {
						break
					}
				}
			} else {
				p, err := reader.ReadString(0)
				path = p
				if err == io.EOF {
					isEof = true
				}
			}

			if path == "" {
				os.Exit(1)
			}

			f := git.File(path)
			if s, submodule, _ := f.IsInSubmodule(c); s {
				fmt.Fprintf(os.Stderr, "fatal: Pathspec '%v' is in submodule '%s'\n", path, submodule)
				os.Exit(128)
			}
			if i, _ := f.IsInsideSymlink(); i {
				fmt.Fprintf(os.Stderr, "fatal: pathspec '%v' is beyond a symbolic link", path)
				os.Exit(128)
			}

			// Invoke the ignore routines directly, rather than relying on
			//  LsFiles because we need to be able to do things such as
			//  return non-matches and match details that aren't supported there.
			// Note that check-ignore has no choice but to use standard ignore
			//  patterns. There is no way to specify custom patterns on the command-line.
			standardIgnores, err := git.StandardIgnorePatterns(c, []git.File{git.File(path)})
			if err != nil {
				return err
			}

			matches, err := git.MatchIgnores(c, standardIgnores, []git.File{git.File(path)})
			if err != nil {
				return err
			}

			match := matches[0]

			// Zero out any match pattern if it's in the index and the no-index options was
			//  not specified.
			if !noIndex {
				entries, _ := git.LsFiles(c, git.LsFilesOptions{Cached: true, Others: false, ExcludeStandard: false}, []git.File{git.File(path)})
				if len(entries) > 0 {
					match.IgnorePattern = git.IgnorePattern{}
				}
			}

			if match.Pattern != "" || nonMatch {
				if !quiet && !verbose {
					fmt.Printf("%s", match.PathName.String())
					if !machine {
						fmt.Printf("\n")
					} else {
						os.Stdout.Write([]byte{0})
					}
				} else if !quiet && verbose && !machine {
					fmt.Printf("%s\n", match)
				} else if !quiet && verbose && machine {
					fmt.Printf("%s", match.Source)
					os.Stdout.Write([]byte{0})
					fmt.Printf("%s", match.LineString())
					os.Stdout.Write([]byte{0})
					fmt.Printf("%s", match.Pattern)
					os.Stdout.Write([]byte{0})
					fmt.Printf("%s", match.PathName)
				}

				if match.Pattern != "" {
					exitCode = 0
				}
			}

			if isEof {
				os.Exit(exitCode)
			}
		}
	}
	return nil
}
