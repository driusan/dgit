package git

import (
	"fmt"
	"log"
	"os"
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
	// (Not implemented)
	Prefix string

	// Not implemented
	ExcludePerDirectory string

	// Name of the index file to use under c.GitDir
	IndexOutput string

	// Discard all the entries in the index instead of updating it to the
	// named Treeish.
	Empty bool

	// Not implemented
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

	// Remove duplicates
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

	if err := checkMergeAndUpdate(c, opt, origMap, idx); err != nil {
		return nil, err
	}

	return idx, readtreeSaveIndex(c, opt, idx)
}

// ReadTreeFastForward will return a new Index with parent fast-forwarded to
// from parent to dst. Local modifications to the work tree will be preserved.
// If options.DryRun is not false, it will also be written to the Client's index file.
func ReadTreeFastForward(c *Client, opt ReadTreeOptions, parent, dst Treeish) (*Index, error) {
	// First do some sanity checks
	if opt.Update && opt.Prefix == "" && !opt.Merge && !opt.Reset {
		return nil, fmt.Errorf("-u is meaningless without -m, --reset or --prefix")
	}
	if opt.Prefix != "" {
		return nil, fmt.Errorf("--prefix is not yet implemented")
	}

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
			return nil, fmt.Errorf("Could not fast-forward. (Case 8-9.)")
		} else if HExists && !MExists {
			if pathname.IsClean(c, IEntry.Sha1) && IEntry.Sha1 == HEntry.Sha1 {
				// Case 10. Remove from the index.
				// (Since it's a new index, we just don't add it)
				continue
			}
			// Case 11 or 13 if it's not clean, case 12 if they don't match
			return nil, fmt.Errorf("Could not fast-forward (case 11-13)")
		} else {
			if HEntry.Sha1 == MEntry.Sha1 {
				// Case 14-15
				newidx.Objects = append(newidx.Objects, IEntry)
				continue
			}
			// H != M
			if IEntry.Sha1 != HEntry.Sha1 && IEntry.Sha1 != MEntry.Sha1 {
				// Case 16-17
				return nil, fmt.Errorf("Could not fast-forward (case 16-17.)")
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
					return nil, fmt.Errorf("Could not fast-forward (case 21.)")
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

	// There's only case 2 left. Case 2 resolves to "remove from index."
	// Since we never added it to newidx, it's already removed. We don't
	// need to range over H to verify that.

	// We need to make sure the number of index entries stays is correct,
	// it's going to be an invalid index..
	newidx.NumberIndexEntries = uint32(len(newidx.Objects))
	if err := checkMergeAndUpdate(c, opt, I, newidx); err != nil {
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

// Reads a tree into the index. If DryRun is not false, it will also be written
// to disk.
func ReadTree(c *Client, opt ReadTreeOptions, tree Treeish) (*Index, error) {
	if opt.Prefix != "" {
		return nil, fmt.Errorf("--prefix is not yet implemented")
	}
	idx, _ := c.GitDir.ReadIndex()
	origMap := idx.GetMap()

	// Convert to a new map before doing anything, so that checkMergeAndUpdate
	// can compare the original update after we reset.
	if opt.Empty {
		idx.NumberIndexEntries = 0
		idx.Objects = make([]*IndexEntry, 0)
		if err := checkMergeAndUpdate(c, opt, origMap, idx); err != nil {
			return nil, err
		}
		return idx, readtreeSaveIndex(c, opt, idx)
	}
	newidx := NewIndex()
	if err := newidx.ResetIndex(c, tree); err != nil {
		return nil, err
	}
	for _, entry := range newidx.Objects {
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

	if err := checkMergeAndUpdate(c, opt, origMap, newidx); err != nil {
		return nil, err
	}
	return idx, readtreeSaveIndex(c, opt, newidx)
}

// Check if the merge would overwrite any modified files and return an error if so (unless --reset),
// then update the file system.
func checkMergeAndUpdate(c *Client, opt ReadTreeOptions, origidx map[IndexPath]*IndexEntry, newidx *Index) error {
	if opt.Update && opt.Prefix == "" && !opt.Merge && !opt.Reset {
		return fmt.Errorf("-u is meaningless without -m, --reset or --prefix")
	}
	if (opt.Prefix != "" && (opt.Merge || opt.Reset)) ||
		(opt.Merge && (opt.Prefix != "" || opt.Reset)) ||
		(opt.Reset && (opt.Prefix != "" || opt.Merge)) {
		return fmt.Errorf("Can only specify one of -u, --reset, or --prefix")
	}

	// Keep a list of index entries to be updated by CheckoutIndex.
	files := make([]File, 0, len(newidx.Objects))
	filemap := make(map[File]*IndexEntry)

	if opt.Merge || opt.Reset {
		// Verify that merge won't overwrite anything that's been modified locally.
		for _, entry := range newidx.Objects {
			f, err := entry.PathName.FilePath(c)
			if err != nil {
				return err
			}

			if f.IsDir() && opt.Update {
				untracked, err := LsFiles(c, LsFilesOptions{Others: true}, []File{f})
				if err != nil {
					return err
				}
				if len(untracked) > 0 {
					return fmt.Errorf("error: Updating '%s%s' would lose untracked files in it", c.SuperPrefix, entry.PathName)
				} else {
					os.RemoveAll(f.String())
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
			orig, ok := origidx[entry.PathName]
			if !ok {
				// If it wasn't in the original index, make sure
				// we check it out.
				file, err := entry.PathName.FilePath(c)
				if err != nil {
					return err
				}
				files = append(files, file)
				filemap[file] = entry
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
				filemap[file] = entry
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
					filemap[file] = entry
				}
			}
		}
	}

	if opt.Update || opt.Reset {
		if err := CheckoutIndexUncommited(c, newidx, CheckoutIndexOptions{Quiet: true, Force: true}, files); err != nil {
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
			// It was deleted from the new index, but was in the
			// original index, so delete it if it hasn't been
			// changed on the filesystem.
			if path.IsClean(c, entry.Sha1) {
				file, err := path.FilePath(c)
				if err != nil {
					// Don't error out since we've already
					// mucked up other stuff, just carry
					// on to the next file.
					fmt.Fprintf(os.Stderr, "%v\n", err)
					continue

				}
				if err := file.Remove(); err != nil {
					fmt.Fprintf(os.Stderr, "%v\n", err)
				}
			}
		}

		// Update stat information for things changed by CheckoutIndex.
		for _, entry := range newidx.Objects {
			if err := entry.RefreshStat(c); err != nil {
				// The error is likely just "no such file or directory", but
				// trace it just in case.
				log.Println(err)
			}
		}
	}
	return nil
}
