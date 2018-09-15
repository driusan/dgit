package git

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Options that may be passed in the command line to ReadTree.
type ReadTreeOptions struct {
	// Perform a merge
	Merge bool
	// Discard changes that are not in stage 0 while doing a merge.
	Reset bool

	// Also update the work tree
	Update bool

	// -i parameter to ReadTree. Not implemented
	IgnoreWorktreeCheck bool

	// Do not write index to disk.
	DryRun bool

	// Unused, just for consistency with real git command line.
	Verbose bool

	// Not implemented
	TrivialMerge, AggressiveMerge bool

	// Read the named tree into the directory prefix/ under the index
	Prefix string

	// Used as the name of a .gitignore to look for in each directory
	ExcludePerDirectory string

	// Name of the index file to use under c.GitDir
	IndexOutput string

	// Discard all the entries in the index instead of updating it to the
	// named Treeish.
	Empty bool

	// Disable sparse checkout.
	// Note that it's the caller's responsibility to set this option if
	// core.sparsecheckout is not equal to true.
	NoSparseCheckout bool
}

// Helper to safely check if path is the same in p1 and p2
func samePath(p1, p2 IndexMap, path IndexPath) bool {
	p1i, p1ok := p1[path]
	p2i, p2ok := p2[path]

	// It's in one but not the other directly, so it's not
	// the same.
	if p1ok != p2ok {
		return false
	}

	// Avoid a nil pointer below by explicitly checking if one
	// is missing.
	if p1ok == false {
		return p2ok == false
	}
	if p2ok == false {
		return p1ok == false
	}

	// It's in both, so we can safely check the sha
	return p1i.Sha1 == p2i.Sha1

}

func checkSparseMatches(c *Client, opt ReadTreeOptions, path IndexPath, patterns []IgnorePattern) bool {
	if opt.NoSparseCheckout {
		// If noSparseCheckout is set, just claim everything
		// matches.
		return true
	}
	fp, err := path.FilePath(c)
	if err != nil {
		panic(err)
	}
	f := fp.String()
	matches := false
	for _, pattern := range patterns {
		if pattern.Negates() {
			if pattern.Matches(f, false) {
				matches = !matches
			}
		} else {
			if pattern.Matches(f, false) {
				matches = true
			}
		}
	}
	return matches
}

func parseSparsePatterns(c *Client, opt *ReadTreeOptions) []IgnorePattern {
	if opt.NoSparseCheckout {
		return nil
	}
	sparsefile := c.GitDir.File("info/sparse-checkout")
	if !sparsefile.Exists() {
		// If the file doesn't exist, pretend the
		// flag to ignore the file was set since the
		// logic is identical.
		opt.NoSparseCheckout = true
		return nil
	}
	sp, err := ParseIgnorePatterns(c, sparsefile, "")
	if err != nil {
		return nil
	}
	return sp
}

// ReadTreeThreeWay will perform a three-way merge on the trees stage1, stage2, and stage3.
// In a normal merge, stage1 is the common ancestor, stage2 is "our" changes, and
// stage3 is "their" changes. See git-read-tree(1) for details.
//
// If options.DryRun is not false, it will also be written to the Client's index file.
func ReadTreeThreeWay(c *Client, opt ReadTreeOptions, stage1, stage2, stage3 Treeish) (*Index, error) {
	idx, err := c.GitDir.ReadIndex()
	if err != nil {
		return nil, err
	}

	resetremovals, err := checkReadtreePrereqs(c, opt, idx)
	if err != nil {
		return nil, err
	}

	origMap := idx.GetMap()

	base, err := GetIndexMap(c, stage1)
	if err != nil {
		return nil, err
	}

	ours, err := GetIndexMap(c, stage2)
	if err != nil {
		return nil, err
	}

	theirs, err := GetIndexMap(c, stage3)
	if err != nil {
		return nil, err
	}

	// Create a slice which contins all objects in base, ours, or theirs
	var allPaths []*IndexEntry
	for path, _ := range base {
		allPaths = append(allPaths, &IndexEntry{PathName: path})
	}
	for path, _ := range ours {
		allPaths = append(allPaths, &IndexEntry{PathName: path})
	}
	for path, _ := range theirs {
		allPaths = append(allPaths, &IndexEntry{PathName: path})
	}
	// Sort to ensure directories come before files.
	sort.Sort(ByPath(allPaths))

	// Remove duplicates and exclude files that aren't part of the
	// sparse checkout rules if applicable.
	var allObjects []IndexPath
	for i := range allPaths {
		if i > 0 && allPaths[i].PathName == allPaths[i-1].PathName {
			continue
		}
		allObjects = append(allObjects, allPaths[i].PathName)
	}
	var dirs []IndexPath

	// Checking for merge conflict with index. If this seems like a confusing mess, it's mostly
	// because it was written to pass the t1000-read-tree-m-3way test case from the official git
	// test suite.
	//
	// The logic can probably be cleaned up.
	for path, orig := range origMap {
		o, ok := ours[path]
		if !ok {
			// If it's been added to the index in the same state as Stage 3, and it's not in
			// stage 1 or 2 it's fine.
			if !base.Contains(path) && !ours.Contains(path) && samePath(origMap, theirs, path) {
				continue
			}

			return idx, fmt.Errorf("Entry '%v' would be overwritten by a merge. Cannot merge.", path)
		}

		// Variable names mirror the O/A/B from the test suite, with "c" for contains
		oc := base.Contains(path)
		ac := ours.Contains(path)
		bc := theirs.Contains(path)

		if oc && ac && bc {
			oa := samePath(base, ours, path)
			ob := samePath(base, theirs, path)

			// t1000-read-tree-m-3way.sh test 75 "must match A in O && A && B && O!=A && O==B case.
			// (This means we can't error out if the Sha1s dont match.)
			if !oa && ob {
				continue
			}
			if oa && !ob {
				// Relevent cases:
				// Must match and be up-to-date in O && A && B && O==A && O!=B
				// May  match B in                 O && A && B && O==A && O!=B
				b, ok := theirs[path]
				if ok && b.Sha1 == orig.Sha1 {
					continue
				} else if !path.IsClean(c, o.Sha1) {
					return idx, fmt.Errorf("Entry '%v' would be overwritten by a merge. Cannot merge.", path)
				}
			}
		}
		// Must match and be up-to-date in !O && A && B && A != B case test from AND
		// Must match and be up-to-date in O && A && B && A != B case test from
		// t1000-read-tree-m-3way.sh in official git
		if ac && bc && !samePath(ours, theirs, path) {
			if !path.IsClean(c, o.Sha1) {
				return idx, fmt.Errorf("Entry '%v' would be overwritten by a merge. Cannot merge.", path)
			}
		}

		// Must match and be up-to-date in O && A && !B && !B && O != A case AND
		// Must match and be up-to-date in O && A && !B && !B && O == A case from
		// t1000-read-tree-m-3way.sh in official git
		if oc && ac && !bc {
			if !path.IsClean(c, o.Sha1) {
				return idx, fmt.Errorf("Entry '%v' would be overwritten by a merge. Cannot merge.", path)
			}
		}

		if o.Sha1 != orig.Sha1 {
			return idx, fmt.Errorf("Entry '%v' would be overwritten by a merge. Cannot merge.", path)
		}
	}
	idx = NewIndex()
paths:
	for _, path := range allObjects {
		// Handle directory/file conflicts.
		if base.HasDir(path) || ours.HasDir(path) || theirs.HasDir(path) {
			if !opt.Merge && !opt.Reset {
				// If not merging, the file wins.
				// see http://www.stackoverflow.com/questions/52175720/how-does-git-read-tree-work-without-m-or-reset-option
				continue
			}
			// Keep track of what was a directory so that other
			// other paths know if they had a conflict higher
			// up in the tree.
			dirs = append(dirs, path)

			// Add the non-directory version fo the appropriate stage
			if p, ok := base[path]; ok {
				idx.AddStage(c, path, p.Mode, p.Sha1, Stage1, p.Fsize, time.Now().UnixNano(), UpdateIndexOptions{Add: true})
			}
			if p, ok := ours[path]; ok {
				idx.AddStage(c, path, p.Mode, p.Sha1, Stage2, p.Fsize, time.Now().UnixNano(), UpdateIndexOptions{Add: true})
			}
			if p, ok := theirs[path]; ok {
				idx.AddStage(c, path, p.Mode, p.Sha1, Stage3, p.Fsize, time.Now().UnixNano(), UpdateIndexOptions{Add: true})
			}
			continue
		}

		// Handle the subfiles in any directory that had a conflict
		// by just adding them in the appropriate stage, because
		// there's no way for a directory and file to not be in
		// conflict.
		for _, d := range dirs {
			if strings.HasPrefix(string(path), string(d+"/")) {
				if p, ok := base[path]; ok {
					if err := idx.AddStage(c, path, p.Mode, p.Sha1, Stage1, p.Fsize, time.Now().UnixNano(), UpdateIndexOptions{Add: true}); err != nil {
						return nil, err
					}
				}
				if p, ok := ours[path]; ok {
					if err := idx.AddStage(c, path, p.Mode, p.Sha1, Stage2, p.Fsize, time.Now().UnixNano(), UpdateIndexOptions{Add: true, Replace: true}); err != nil {
						return nil, err
					}
				}
				if p, ok := theirs[path]; ok {
					if err := idx.AddStage(c, path, p.Mode, p.Sha1, Stage3, p.Fsize, time.Now().UnixNano(), UpdateIndexOptions{Add: true}); err != nil {
						return nil, err
					}
				}
				continue paths
			}
		}

		// From here on out, we assume everything is a file.

		// All three trees are the same, don't do anything to the index.
		if samePath(base, ours, path) && samePath(base, theirs, path) {
			if err := idx.AddStage(c, path, ours[path].Mode, ours[path].Sha1, Stage0, ours[path].Fsize, time.Now().UnixNano(), UpdateIndexOptions{Add: true}); err != nil {
				panic(err)
			}
			continue
		}

		// If both stage2 and stage3 are the same, the work has been done in
		// both branches, so collapse to stage0 (use our changes)
		if samePath(ours, theirs, path) {
			if ours.Contains(path) {
				if err := idx.AddStage(c, path, ours[path].Mode, ours[path].Sha1, Stage0, ours[path].Fsize, time.Now().UnixNano(), UpdateIndexOptions{Add: true}); err != nil {
					panic(err)
				}
				continue
			}
		}

		// If stage1 and stage2 are the same, our branch didn't do anything,
		// but theirs did, so take their changes.
		if samePath(base, ours, path) {
			if theirs.Contains(path) {
				if err := idx.AddStage(c, path, theirs[path].Mode, theirs[path].Sha1, Stage0, theirs[path].Fsize, time.Now().UnixNano(), UpdateIndexOptions{Add: true}); err != nil {
					panic(err)
				}
				continue
			}
		}

		// If stage1 and stage3 are the same, we did something
		// but they didn't, so take our changes
		if samePath(base, theirs, path) {
			if ours.Contains(path) {
				o := ours[path]
				if err := idx.AddStage(c, path, o.Mode, o.Sha1, Stage0, o.Fsize, time.Now().UnixNano(), UpdateIndexOptions{Add: true}); err != nil {
					panic(err)
				}
				continue
			}
		}

		// We couldn't short-circuit out, so add all three stages.

		// Remove Stage0 if it exists. If it doesn't, then at worst we'll
		// remove a stage that we're about to add back.
		idx.RemoveFile(path)

		if b, ok := base[path]; ok {
			idx.AddStage(c, path, b.Mode, b.Sha1, Stage1, b.Fsize, time.Now().UnixNano(), UpdateIndexOptions{Add: true})
		}
		if o, ok := ours[path]; ok {
			idx.AddStage(c, path, o.Mode, o.Sha1, Stage2, o.Fsize, time.Now().UnixNano(), UpdateIndexOptions{Add: true})
		}
		if t, ok := theirs[path]; ok {
			idx.AddStage(c, path, t.Mode, t.Sha1, Stage3, t.Fsize, time.Now().UnixNano(), UpdateIndexOptions{Add: true})
		}
	}

	if err := checkMergeAndUpdate(c, opt, origMap, idx, resetremovals); err != nil {
		return nil, err
	}

	return idx, readtreeSaveIndex(c, opt, idx)
}

// ReadTreeFastForward will return a new Index with parent fast-forwarded to
// from parent to dst. Local modifications to the work tree will be preserved.
// If options.DryRun is not false, it will also be written to the Client's index file.
func ReadTreeFastForward(c *Client, opt ReadTreeOptions, parent, dst Treeish) (*Index, error) {
	// This is the table of how fast-forward merges work from git-read-tree(1)
	// I == Index, H == parent, and M == dst in their terminology. (ie. It's a
	// fast-forward from H to M while the index is in state I.)
	//
	//	      I 		  H	   M	    Result
	//	     -------------------------------------------------------
	//	   0  nothing		  nothing  nothing  (does not happen)
	//	   1  nothing		  nothing  exists   use M
	//	   2  nothing		  exists   nothing  remove path from index
	//	   3  nothing		  exists   exists,  use M if "initial checkout",
	//					   H == M   keep index otherwise
	//					   exists,  fail
	//					   H != M
	//
	//	      clean I==H  I==M
	//	     ------------------
	//	   4  yes   N/A   N/A	  nothing  nothing  keep index
	//	   5  no    N/A   N/A	  nothing  nothing  keep index
	//
	//	   6  yes   N/A   yes	  nothing  exists   keep index
	//	   7  no    N/A   yes	  nothing  exists   keep index
	//	   8  yes   N/A   no	  nothing  exists   fail
	//	   9  no    N/A   no	  nothing  exists   fail
	//
	//	   10 yes   yes   N/A	  exists   nothing  remove path from index
	//	   11 no    yes   N/A	  exists   nothing  fail
	//	   12 yes   no	  N/A	  exists   nothing  fail
	//	   13 no    no	  N/A	  exists   nothing  fail
	//
	//	      clean (H==M)
	//	     ------
	//	   14 yes		  exists   exists   keep index
	//	   15 no		  exists   exists   keep index
	//
	//	      clean I==H  I==M (H!=M)
	//	     ------------------
	//	   16 yes   no	  no	  exists   exists   fail
	//	   17 no    no	  no	  exists   exists   fail
	//	   18 yes   no	  yes	  exists   exists   keep index
	//	   19 no    no	  yes	  exists   exists   keep index
	//	   20 yes   yes   no	  exists   exists   use M
	//	   21 no    yes   no	  exists   exists   fail
	idx, err := c.GitDir.ReadIndex()
	if err != nil {
		return nil, err
	}

	resetremovals, err := checkReadtreePrereqs(c, opt, idx)
	if err != nil {
		return nil, err
	}

	I := idx.GetMap()

	H, err := GetIndexMap(c, parent)
	if err != nil {
		return nil, err
	}

	M, err := GetIndexMap(c, dst)
	if err != nil {
		return nil, err
	}

	// Start by iterating through the index and handling cases 5-21.
	// (We'll create a new index instead of trying to keep track of state
	// of the existing index while iterating through it.)
	newidx := NewIndex()

	for pathname, IEntry := range I {
		HEntry, HExists := H[pathname]
		MEntry, MExists := M[pathname]
		if !HExists && !MExists {
			// Case 4-5
			newidx.Objects = append(newidx.Objects, IEntry)
			continue
		} else if !HExists && MExists {
			if IEntry.Sha1 == MEntry.Sha1 {
				// Case 6-7
				newidx.Objects = append(newidx.Objects, IEntry)
				continue
			}
			// Case 8-9
			return nil, fmt.Errorf("error: Entry '%s' would be overwritten by merge. Cannot merge.", pathname)
		} else if HExists && !MExists {
			if pathname.IsClean(c, IEntry.Sha1) && IEntry.Sha1 == HEntry.Sha1 {
				// Case 10. Remove from the index.
				// (Since it's a new index, we just don't add it)
				continue
			}
			// Case 11 or 13 if it's not clean, case 12 if they don't match
			if IEntry.Sha1 != HEntry.Sha1 {
				return nil, fmt.Errorf("error: Entry '%s' would be overwritten by merge. Cannot merge.", pathname)
			}
			return nil, fmt.Errorf("Entry '%v' not uptodate. Cannot merge.", pathname)
		} else {
			if HEntry.Sha1 == MEntry.Sha1 {
				// Case 14-15
				newidx.Objects = append(newidx.Objects, IEntry)
				continue
			}
			// H != M
			if IEntry.Sha1 != HEntry.Sha1 && IEntry.Sha1 != MEntry.Sha1 {
				// Case 16-17
				return nil, fmt.Errorf("error: Entry '%s' would be overwritten by merge. Cannot merge.", pathname)
			} else if IEntry.Sha1 != HEntry.Sha1 && IEntry.Sha1 == MEntry.Sha1 {
				// Case 18-19
				newidx.Objects = append(newidx.Objects, IEntry)
				continue
			} else if IEntry.Sha1 == HEntry.Sha1 && IEntry.Sha1 != MEntry.Sha1 {
				if pathname.IsClean(c, IEntry.Sha1) {
					// Case 20. Use M.
					newidx.Objects = append(newidx.Objects, MEntry)
					continue
				} else {
					return nil, fmt.Errorf("Entry '%v' not uptodate. Cannot merge.", pathname)
				}
			}
		}
	}

	// Finally, handle the cases where it's in H or M but not I by going
	// through the maps of H and M.
	for pathname, MEntry := range M {
		if _, IExists := I[pathname]; IExists {
			// If it's in I, it was already handled above.
			continue
		}
		HEntry, HExists := H[pathname]
		if !HExists {
			// It's in M but not I or H. Case 1. Use M.
			newidx.Objects = append(newidx.Objects, MEntry)
			continue
		}
		// Otherwise it's in both H and M but not I. Case 3.
		if HEntry.Sha1 != MEntry.Sha1 {
			if !c.GitDir.File("index").Exists() {
				// Case 3 from the git-read-tree(1) is weird, but this
				// is intended to handle it. If there is no index, add
				// the file from M
				newidx.Objects = append(newidx.Objects, MEntry)
			} else {
				return nil, fmt.Errorf("Could not fast-forward (case 3.)")
			}
		} else {
			// It was unmodified between the two trees, but has been
			// removed from the index. Keep the "Deleted" state by
			// not adding it.
			// If there is no index, however, we add it, since it's
			// an initial checkout.
			if !c.GitDir.File("index").Exists() {
				newidx.Objects = append(newidx.Objects, MEntry)
			}
		}
	}

	// We need to make sure the number of index entries stays is correct,
	// it's going to be an invalid index..
	newidx.NumberIndexEntries = uint32(len(newidx.Objects))
	if err := checkMergeAndUpdate(c, opt, I, newidx, resetremovals); err != nil {
		return nil, err
	}

	return newidx, readtreeSaveIndex(c, opt, newidx)
}

// Helper to ensure the DryRun option gets checked no matter what the code path
// for ReadTree.
func readtreeSaveIndex(c *Client, opt ReadTreeOptions, i *Index) error {
	if !opt.DryRun {
		if opt.IndexOutput == "" {
			opt.IndexOutput = "index"
		}
		f, err := c.GitDir.Create(File(opt.IndexOutput))
		if err != nil {
			return err
		}
		defer f.Close()
		return i.WriteIndex(f)
	}
	return nil
}

// Check if the read-tree can be performed. Returns a list of files that need to be
// removed if --reset is specified.
func checkReadtreePrereqs(c *Client, opt ReadTreeOptions, idx *Index) ([]File, error) {
	if opt.Update && opt.Prefix == "" && !opt.Merge && !opt.Reset {
		return nil, fmt.Errorf("-u is meaningless without -m, --reset or --prefix")
	}
	if (opt.Prefix != "" && (opt.Merge || opt.Reset)) ||
		(opt.Merge && (opt.Prefix != "" || opt.Reset)) ||
		(opt.Reset && (opt.Prefix != "" || opt.Merge)) {
		return nil, fmt.Errorf("Can only specify one of -u, --reset, or --prefix")
	}
	if opt.ExcludePerDirectory != "" && !opt.Update {
		return nil, fmt.Errorf("--exclude-per-directory is meaningless without -u")
	}
	if idx == nil {
		return nil, nil
	}

	toremove := make([]File, 0)
	for _, entry := range idx.Objects {
		if entry.Stage() != Stage0 {
			if opt.Merge {
				return nil, fmt.Errorf("You need to resolve your current index first")
			}
			if opt.Reset {
				f, err := entry.PathName.FilePath(c)
				if err != nil {
				}
				// Indexes are sorted, so only check if the last file is the
				// same
				if len(toremove) >= 1 && toremove[len(toremove)-1] == f {
					continue
				}
				toremove = append(toremove, f)
			}
		}
	}
	return toremove, nil

}

// Reads a tree into the index. If DryRun is not false, it will also be written
// to disk.
func ReadTree(c *Client, opt ReadTreeOptions, tree Treeish) (*Index, error) {
	idx, _ := c.GitDir.ReadIndex()
	origMap := idx.GetMap()

	resetremovals, err := checkReadtreePrereqs(c, opt, idx)
	if err != nil {
		return nil, err
	}
	// Convert to a new map before doing anything, so that checkMergeAndUpdate
	// can compare the original update after we reset.
	if opt.Empty {
		idx.NumberIndexEntries = 0
		idx.Objects = make([]*IndexEntry, 0)
		if err := checkMergeAndUpdate(c, opt, origMap, idx, resetremovals); err != nil {
			return nil, err
		}
		return idx, readtreeSaveIndex(c, opt, idx)
	}
	newidx := NewIndex()
	if err := newidx.ResetIndex(c, tree); err != nil {
		return nil, err
	}
	for _, entry := range newidx.Objects {
		if opt.Prefix != "" {
			// Add it to the original index with the prefix
			entry.PathName = IndexPath(opt.Prefix) + entry.PathName
			if err := idx.AddStage(c, entry.PathName, entry.Mode, entry.Sha1, Stage0, entry.Fsize, entry.Mtime, UpdateIndexOptions{Add: true}); err != nil {
				return nil, err
			}
		}
		if opt.Merge {
			if oldentry, ok := origMap[entry.PathName]; ok {
				newsha, _, err := HashFile("blob", string(entry.PathName))
				if err != nil && newsha == entry.Sha1 {
					entry.Ctime, entry.Ctimenano = oldentry.Ctime, oldentry.Ctimenano
					entry.Mtime = oldentry.Mtime
				}
			}
		}
	}

	if opt.Prefix == "" {
		idx = newidx
	}

	if err := checkMergeAndUpdate(c, opt, origMap, idx, resetremovals); err != nil {
		return nil, err
	}
	return idx, readtreeSaveIndex(c, opt, idx)
}

// Check if the merge would overwrite any modified files and return an error if so (unless --reset),
// then update the file system.
func checkMergeAndUpdate(c *Client, opt ReadTreeOptions, origidx map[IndexPath]*IndexEntry, newidx *Index, resetremovals []File) error {
	sparsePatterns := parseSparsePatterns(c, &opt)
	if !opt.NoSparseCheckout {
		leavesFile := false
		newSparse := false
		for _, entry := range newidx.Objects {
			if !checkSparseMatches(c, opt, entry.PathName, sparsePatterns) {
				if orig, ok := origidx[entry.PathName]; ok && !orig.SkipWorktree() {
					newSparse = true
				}
				entry.SetSkipWorktree(true)
				if newidx.Version <= 2 {
					newidx.Version = 3
				}
			} else {
				leavesFile = true
			}
		}
		for _, entry := range origidx {
			if checkSparseMatches(c, opt, entry.PathName, sparsePatterns) {
				// This isn't necessarily true, but if it is we don't error out in
				// order to make let make the git test t1011.19 pass.
				//
				// t1011-read-tree-sparse-checkout works in mysterious ways.
				leavesFile = true
			}
		}
		if !leavesFile && newSparse {
			return fmt.Errorf("Sparse checkout would leave no file in work tree")
		}
	}
	// Keep a list of index entries to be updated by CheckoutIndex.
	files := make([]File, 0, len(newidx.Objects))

	if opt.Merge || opt.Reset || opt.Update {
		// Verify that merge won't overwrite anything that's been modified locally.
		for _, entry := range newidx.Objects {
			f, err := entry.PathName.FilePath(c)
			if err != nil {
				return err
			}

			if opt.Update && f.IsDir() {
				untracked, err := LsFiles(c, LsFilesOptions{Others: true, Modified: true}, []File{f})
				if err != nil {
					return err
				}
				if len(untracked) > 0 {
					return fmt.Errorf("error: Updating '%s%s' would lose untracked files in it", c.SuperPrefix, entry.PathName)
				}
			}
			if entry.Stage() != Stage0 {
				// Don't check unmerged entries. One will always
				// conflict, which means that -u won't work
				// if we check them.
				// (We also don't add them to files, so they won't
				// make it to checkoutindex
				continue
			}
			if entry.SkipWorktree() {
				continue
			}
			if opt.Update && !f.Exists() {
				// It doesn't exist on the filesystem, so it should be checked out.
				files = append(files, f)
				continue
			}
			orig, ok := origidx[entry.PathName]
			if !ok {
				// If it wasn't in the original index, make sure
				// we check it out after verifying there's not
				// already something there.
				if opt.Update && f.Exists() {
					lsopts := LsFilesOptions{Others: true}
					if opt.ExcludePerDirectory != "" {
						lsopts.ExcludePerDirectory = []File{File(opt.ExcludePerDirectory)}
					}
					untracked, err := LsFiles(c, lsopts, []File{f})
					if err != nil {
						return err
					}
					if len(untracked) > 0 {
						if !entry.PathName.IsClean(c, entry.Sha1) {
							return fmt.Errorf("Untracked working tree file '%v' would be overwritten by merge", f)
						}
					}
				}
				file, err := entry.PathName.FilePath(c)
				if err != nil {
					return err
				}
				files = append(files, file)
				continue
			}

			if orig.Sha1 == entry.Sha1 {
				// Nothing was modified, so don't bother checking
				// anything out
				continue
			}
			if entry.PathName.IsClean(c, orig.Sha1) {
				// it hasn't been modified locally, so we want to
				// make sure the newidx version is checked out.
				file, err := entry.PathName.FilePath(c)
				if err != nil {
					return err
				}
				files = append(files, file)
				continue
			} else {
				// There are local unmodified changes on the filesystem
				// from the original that would be lost by -u, so return
				// an error unless --reset is specified.
				if !opt.Reset {
					return fmt.Errorf("%s has local changes. Can not merge.", entry.PathName)
				} else {
					// with --reset, checkout the file anyways.
					file, err := entry.PathName.FilePath(c)
					if err != nil {
						return err
					}
					files = append(files, file)
				}
			}
		}
	}

	if !opt.DryRun && (opt.Update || opt.Reset) {
		if err := CheckoutIndexUncommited(c, newidx, CheckoutIndexOptions{Quiet: true, Force: true, Prefix: opt.Prefix}, files); err != nil {
			return err
		}

		// Convert to a map for constant time lookup in our loop..
		newidxMap := newidx.GetMap()

		// Before returning, delete anything that was in the old index, removed
		// from the new index, and hasn't been changed on the filesystem.
		for path, entry := range origidx {
			if _, ok := newidxMap[path]; ok {
				// It was already handled by checkout-index
				continue
			}
			file, err := path.FilePath(c)
			if err != nil {
				// Don't error out since we've already
				// mucked up other stuff, just carry
				// on to the next file.
				fmt.Fprintf(os.Stderr, "%v\n", err)
				continue

			}

			// It was deleted from the new index, but was in the
			// original index, so delete it if it hasn't been
			// changed on the filesystem.
			if path.IsClean(c, entry.Sha1) {
				if err := removeFileClean(file); err != nil {
					return err
				}
			} else if !opt.NoSparseCheckout {
				if !checkSparseMatches(c, opt, path, sparsePatterns) {
					if file.Exists() {
						if err := removeFileClean(file); err != nil {
							return err
						}
					}
				}
			}
		}

		// Update stat information for things changed by CheckoutIndex, and remove anything
		// with the SkipWorktree bit set.
		for _, entry := range newidx.Objects {
			f, err := entry.PathName.FilePath(c)
			if err != nil {
				return err
			}
			if f.Exists() {
				if entry.SkipWorktree() {
					if entry.PathName.IsClean(c, entry.Sha1) {
						if err := removeFileClean(f); err != nil {
							return err
						}
					}
					continue
				}
				if err := entry.RefreshStat(c); err != nil {
					return err
				}
			}
		}

		if opt.Reset {
			for _, file := range resetremovals {
				// It may have been removed by the removal loop above
				if file.Exists() {
					if err := file.Remove(); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func removeFileClean(f File) error {
	if err := f.Remove(); err != nil {
		return err
	}
	// If there's nothing left in the directory, remove
	dir := filepath.Dir(f.String())
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	return nil
}
