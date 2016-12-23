package git

import (
	"fmt"
)

type ReadTreeOptions struct {
	// Perform a merge
	Merge bool
	// Discard changes that are not in stage 0.
	Reset bool

	// Also update the work tree
	Update bool

	// -i parameter to ReadTree. Not implemented
	IgnoreWorktreeCheck bool

	// Do not write index to disk.
	DryRun bool

	// Not implemented.
	Verbose bool

	// Not implemented
	TrivialMerge, AggressiveMerge bool

	// Not implemented
	Prefix string

	// Not implemented
	ExcludePerDirectory string

	// Name of the index file to use under c.GitDir (or prefix if specified)
	IndexOutput string

	// Discard all the entries in the index.
	Empty bool

	// Not implemented
	NoSparseCheckout bool
}

// ReadTreeFastForward will return a new GitIndex with parent fast-forwarded to
// from parent to to. If options.DryRun is not false, it will also be written to
// the Client's index file.
func ReadTreeMerge(c *Client, opt ReadTreeOptions, stage1, stage2, stage3 Treeish) (*Index, error) {
	return nil, fmt.Errorf("ReadTreeMerge not yet implemented")
}

// ReadTreeFastForward will return a new Index with parent fast-forwarded to
// from parent to dst. Local modifications to the work tree will be preserved.
// If options.DryRun is not false, it will also be written to the Client's index file.
func ReadTreeFastForward(c *Client, opt ReadTreeOptions, parent, dst Treeish) (*Index, error) {
	return nil, fmt.Errorf("ReadTreeFastForward not yet implemented")
}

// Helper to ensure the DryRun option gets checked no matter what the code path
// for ReadTree.
func readtreeSaveIndex(c *Client, opt ReadTreeOptions, i *Index) error {
	if !opt.DryRun {
		f, err := c.GitDir.Create(File(opt.IndexOutput))
		if err != nil {
			return err
		}
		defer f.Close()
		return i.WriteIndex(f)
	}
	return nil
}

// Reads a tree into the index. If DryRun is not false, it will also be written
// to disk.
func ReadTree(c *Client, opt ReadTreeOptions, tree Treeish) (*Index, error) {
	if opt.Merge || opt.Reset {
		return nil, fmt.Errorf("Must call ReadTreeFastForward or ReadTreeMerge for --merge and --reset options")
	}
	idx, _ := c.GitDir.ReadIndex()

	if opt.Empty {
		idx.NumberIndexEntries = 0
		idx.Objects = make([]*IndexEntry, 0)
		return idx, readtreeSaveIndex(c, opt, idx)
	}
	err := idx.ResetIndex(c, tree)
	if err != nil {
		return nil, err
	}
	return idx, readtreeSaveIndex(c, opt, idx)
}
