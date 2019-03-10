package cmd

import (
	"fmt"
	"os"

	"github.com/driusan/dgit/git"
)

func Fsck(c *git.Client, args []string) error {
	flags := newFlagSet("fsck")
	opts := git.FsckOptions{}

	flags.BoolVar(&opts.Unreachable, "unreachable", false, "Print unreachable objects")
	nodangling := flags.Bool("no-dangling", false, "Do not print dangling objects")
	dangling := flags.Bool("dangling", false, "Print dangling objects (default)")
	flags.BoolVar(&opts.Root, "root", false, "Report root nodes")
	flags.BoolVar(&opts.Tags, "tags", false, "Report tags")
	flags.BoolVar(&opts.Cache, "cache", false, "Consider objects in the index as a head node for unreachability traces")
	flags.BoolVar(&opts.NoReflogs, "no-reflogs", false, "Do not consider commits only reachable from reflogs reachable")
	full := flags.Bool("full", false, "Do not just check GIT_OBJECT_DIRECTORY, but also alternates")
	nofull := flags.Bool("no-full", false, "Disable --full")
	flags.BoolVar(&opts.ConnectivityOnly, "connectivity-only", false, "Only check the connectivity of commits, not the integrity of blobs")
	flags.BoolVar(&opts.Strict, "strict", false, "Check for files with mode g+w bit set")
	flags.BoolVar(&opts.Verbose, "verbose", false, "Be chatty")
	flags.BoolVar(&opts.LostFound, "lost-found", false, "Write dangling objects into .git/lost-found")
	flags.BoolVar(&opts.NameObjects, "name-objects", false, "When displaing names of reachable objects,  also display a name describing how they are reachable compatible with rev-parse")

	noprogress := flags.Bool("no-proress", false, "Do not print progress information")
	progress := flags.Bool("progress", false, "Print progress information (default)")

	flags.Parse(args)

	if *nodangling && *dangling {
		return fmt.Errorf("Can not specify both --dangling and --no-dangling")
	} else if *nodangling {
		opts.NoDangling = true
	} else if *dangling {
		opts.NoDangling = false
	}

	if *noprogress && *progress {
		return fmt.Errorf("Can not specify both --progress and --no-progress")
	} else if *noprogress {
		opts.NoProgress = true
	} else if *progress {
		opts.NoProgress = false
	}

	if *nofull && *full {
		return fmt.Errorf("Can not specify both --full and --no-full")
	} else if *nofull {
		opts.NoFull = true
	} else if *full {
		opts.NoFull = false
	}

	objects := flags.Args()

	// We disregard errs because they've already been printed,
	// they were just returned in case someone wants to use git.Fsck
	// as an API.
	// However, this (cmd.Fsck) still returns an error in case of
	// bad argument parsing.
	errs := git.Fsck(c, os.Stderr, opts, objects)
	if len(errs) > 0 {
		os.Exit(1)
	}
	return nil
}
