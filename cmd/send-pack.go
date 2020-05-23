package cmd

import (
	"flag"
	"fmt"
	//"os"
	//"strings"
	"github.com/driusan/dgit/git"
)

func SendPack(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("send-pack", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	opts := git.SendPackOptions{}
	flags.BoolVar(&opts.All, "all", false, "Update all refs that exist locally")
	flags.BoolVar(&opts.DryRun, "dry-run", false, "Do not send pack")
	flags.BoolVar(&opts.Force, "force", false, "Update remote ref even if not a fast-forward")
	flags.StringVar(&opts.ReceivePack, "receive-pack", "", "Invoke non-standard path to receive-pack on remote")
	flags.BoolVar(&opts.Verbose, "verbose", false, "Be more chatty")
	flags.BoolVar(&opts.Thin, "thin", false, "Send a thin pack")
	flags.BoolVar(&opts.Atomic, "Atomic", false, "Atomicly update refs on remote")
	flags.BoolVar(&opts.Signed, "Signed", false, "GPG sign the push request")
	flags.Parse(args)
	args = flags.Args()
	if len(args) < 1 {
		return fmt.Errorf("Invalid send-pack")
	}
	remote := git.Remote(args[0])
	var refs []git.Refname = make([]git.Refname, 0, len(args)-1)
	for _, ref := range args[1:] {
		refs = append(refs, git.Refname(ref))
	}
	return git.SendPack(c, opts, remote, refs)
}
