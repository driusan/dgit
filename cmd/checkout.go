package cmd

import (
	"fmt"
	"github.com/driusan/go-git/git"
)

// Implements the git checkout command.
//
// BUG(driusan): This needs to be completely rewritten in terms of checkout-index
// and ReadTree, now that they're implemented. CheckoutIndex is much safer and
// more in line with the git client, while this is a hack.
func Checkout(c *git.Client, args []string) error {

	switch len(args) {
	case 0:
		return fmt.Errorf("Missing argument for checkout")
	}

	// Temporary hack until. This needs to parse the arguments with flag..
	return git.Checkout(c, git.CheckoutOptions{}, "HEAD", args)
}
