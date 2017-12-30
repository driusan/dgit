package git

import (
	"bytes"
	"fmt"
	"strings"
)

type WriteTreeOptions struct {
	MissingOk bool
	Prefix    string
}

func WriteTree(c *Client, opts WriteTreeOptions) (Sha1, error) {
	idx, err := c.GitDir.ReadIndex()
	if err != nil {
		return Sha1{}, err
	}

	objs := idx.Objects
	if opts.Prefix != "" {
		opts.Prefix = strings.TrimRight(opts.Prefix, "/")
		// If there's a prefix, take the slice of the index which
		// starts with that prefix.
		prefixStart := -1
		for i, obj := range idx.Objects {
			path := obj.PathName.String()
			if strings.HasPrefix(path, opts.Prefix) {
				if prefixStart < 0 {
					prefixStart = i
				}
			} else {
				if prefixStart >= 0 {
					objs = idx.Objects[prefixStart:i]
					break
				}
			}
		}
		if prefixStart == -1 {
			return Sha1{}, fmt.Errorf("prefix %v not found", opts.Prefix)
		}
	}
	if !opts.MissingOk {
		// Verify that every blob being referenced exists unless "missing-ok" was
		// specified
		for _, obj := range objs {
			ok, _, err := c.HaveObject(obj.Sha1)
			if err != nil {
				return Sha1{}, err
			}
			if !ok {
				return Sha1{}, fmt.Errorf("invalid object %o %v for '%v'", obj.Mode, obj.Sha1, obj.PathName)
			}
		}
	}
	sha1, err := writeTree(c, opts.Prefix, objs)
	if err == ObjectExists {
		return sha1, nil
	}
	return sha1, err
}

func writeTree(c *Client, prefix string, entries []*IndexEntry) (Sha1, error) {
	content := bytes.NewBuffer(nil)
	// [mode] [file/folder name]\0[SHA-1 of referencing blob or tree as [20]byte]

	lastname := ""
	firstIdxForTree := -1

	//	fmt.Printf("Prefix: %v\n", prefix)
	for idx, obj := range entries {
		if obj.Stage() != Stage0 {
			return Sha1{}, fmt.Errorf("Could not write index with unmerged entries")
		}
		relativename := strings.TrimPrefix(obj.PathName.String(), prefix+"/")
		//fmt.Printf("This name: %s\n", relativename)
		nameBits := strings.Split(relativename, "/")

		if len(nameBits) == 0 {
			panic("did not reach base case for subtree recursion")
		} else if len(nameBits) == 1 {
			// Base case: we've recursed all the way down to there being no prefix.
			// write the blob to the tree.
			if firstIdxForTree >= 0 {
				var newPrefix string
				if prefix == "" {
					newPrefix = lastname
				} else {
					newPrefix = prefix + "/" + lastname
				}

				// Get the index entries which compose this tree
				var islice []*IndexEntry
				islice = entries[firstIdxForTree:idx]

				// Get the sha1 for the tree with the prefix stripped.
				subsha1, err := writeTree(c, newPrefix, islice)
				if err != nil && err != ObjectExists {
					panic(err)
				}

				// Write the object
				fmt.Fprintf(content, "%o %s\x00", 0040000, lastname)
				content.Write(subsha1[:])

			}
			fmt.Fprintf(content, "%o %s\x00", obj.Mode, nameBits[0])
			content.Write(obj.Sha1[:])
			lastname = ""
			firstIdxForTree = -1
		} else if (nameBits[0] != lastname && lastname != "") || idx == len(entries)-1 {
			// Either the name of the first part of the directory changed or we've reached
			// the end. Either way, we've found the end of this tree, so write it out.
			var newPrefix string
			if prefix == "" {
				newPrefix = lastname
			} else {
				newPrefix = prefix + "/" + lastname
			}

			// Get the index entries which compose this tree
			var islice []*IndexEntry
			if firstIdxForTree == -1 {
				// There was only 1 entry.
				islice = entries[idx : idx+1]
				if prefix == "" {
					newPrefix = nameBits[0]
				} else {
					newPrefix = prefix + "/" + nameBits[0]
				}
				lastname = nameBits[0]
			} else if idx == len(entries)-1 && nameBits[0] == lastname {
				// We've reached the end, so the slice goes from the first index
				// to the end
				islice = entries[firstIdxForTree:]
			} else {
				// the name changed, so the slice goes from the first index until
				// this index.
				islice = entries[firstIdxForTree:idx]
			}

			// Get the sha1 for the tree with the prefix stripped.
			subsha1, err := writeTree(c, newPrefix, islice)
			if err != nil && err != ObjectExists {
				panic(err)
			}

			// Write the object
			fmt.Fprintf(content, "%o %s\x00", 0040000, lastname)
			content.Write(subsha1[:])

			if idx == len(entries)-1 && lastname != nameBits[0] {
				var newPrefix string
				if prefix == "" {
					newPrefix = nameBits[0]
				} else {
					newPrefix = prefix + "/" + nameBits[0]
				}
				subsha1, err := writeTree(c, newPrefix, entries[len(entries)-1:])
				if err != nil && err != ObjectExists {
					panic(err)
				}

				// Write the object
				fmt.Fprintf(content, "%o %s\x00", 0040000, nameBits[0])
				content.Write(subsha1[:])
			}
			// Reset the data keeping track of what this tree is.
			lastname = nameBits[0]
			firstIdxForTree = idx
		} else {
			// Keep track of the last thing we saw for the next loop iteration.
			lastname = nameBits[0]
			if firstIdxForTree == -1 {
				// This is the first thing we saw since the last tree.
				firstIdxForTree = idx
			}
		}
	}

	return c.WriteObject("tree", content.Bytes())
}
