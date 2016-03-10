package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	libgit "github.com/driusan/git"
	"io"
	"sort"
	//   	"io/ioutil"
	"bytes"
	"crypto/sha1"
	"os"
	"strings"
)

var InvalidIndex error = errors.New("Invalid index")

// index file is defined as Network byte Order (Big Endian)

// 12 byte header:
// 4 bytes: D I R C (stands for "Dir cache")
// 4-byte version number (can be 2, 3 or 4)
// 32bit number of index entries
type fixedGitIndex struct {
	Signature          [4]byte // 4
	Version            uint32  // 8
	NumberIndexEntries uint32  // 12
}

type GitIndex struct {
	fixedGitIndex // 12
	Objects       []*GitIndexEntry
}
type GitIndexEntry struct {
	fixedIndexEntry

	PathName string
}

type fixedIndexEntry struct {
	Ctime     uint32 // 16
	Ctimenano uint32 // 20

	Mtime     uint32 // 24
	Mtimenano uint32 // 28

	Dev uint32 // 32
	Ino uint32 // 36

	Mode uint32 // 40

	Uid uint32 // 44
	Gid uint32 // 48

	Fsize uint32 // 52

	Sha1 [20]byte // 72

	Flags uint16 // 74
}

func ReadIndex(g *libgit.Repository) (*GitIndex, error) {
	file, err := os.Open(g.Path + "/index")
	if err != nil {
		return &GitIndex{
			fixedGitIndex{
				[4]byte{'D', 'I', 'R', 'C'},
				2, // version 2
				0, // no entries
			},
			make([]*GitIndexEntry, 0),
		}, err
	}
	defer file.Close()

	var i fixedGitIndex
	binary.Read(file, binary.BigEndian, &i)
	fmt.Println(i.Signature)
	if i.Signature != [4]byte{'D', 'I', 'R', 'C'} {
		return nil, InvalidIndex
	}
	if i.Version < 2 || i.Version > 4 {
		return nil, InvalidIndex
	}

	var idx uint32
	indexes := make([]*GitIndexEntry, i.NumberIndexEntries, i.NumberIndexEntries)
	for idx = 0; idx < i.NumberIndexEntries; idx += 1 {
		if index, err := ReadIndexEntry(file); err == nil {
			indexes[idx] = index
		}
	}
	return &GitIndex{i, indexes}, nil
}

func ReadIndexEntry(file *os.File) (*GitIndexEntry, error) {
	var f fixedIndexEntry
	var name []byte
	binary.Read(file, binary.BigEndian, &f)

	var nameLength uint16
	nameLength = f.Flags & 0x0FFF

	if nameLength&0xFFF != 0xFFF {
		name = make([]byte, nameLength, nameLength)
		n, err := file.Read(name)
		if err != nil {
			panic("I don't know what to do")
		}
		if n != int(nameLength) {
			panic("Error reading the name")
		}

		// I don't understand where this +4 comes from, but it seems to work
		// out with how the c git implementation calculates the padding..
		//
		// The definition of the index file format at:
		// https://github.com/git/git/blob/master/Documentation/technical/index-format.txt
		// claims that there should be "1-8 nul bytes as necessary to pad the entry to a multiple of eight
		// bytes while keeping the name NUL-terminated."
		//
		// The fixed size of the header is 82 bytes if you add up all the types.
		// the length of the name is nameLength bytes, so according to the spec
		// this *should* be 8 - ((82 + nameLength) % 8) bytes of padding.
		// But reading existant index files, there seems to be an extra 4 bytes
		// incorporated into the index size calculation.
		expectedOffset := 8 - ((82 + nameLength + 4) % 8)
		file.Seek(int64(expectedOffset), 1)
		/*
			This was used to verify that the offset is correct, but it causes problems if the data following
			the offset is empty..
			whitespace := make([]byte, 1, 1)
			var w uint16
			// Read all the whitespace that git uses for alignment.
			for _, _ = file.Read(whitespace); whitespace[0] == 0; _, _ = file.Read(whitespace) {
				w += 1
			}

			if w % 8 != expectedOffset {
				panic(fmt.Sprintf("Read incorrect number of whitespace characters %d vs %d", w, expectedOffset))
			}
			if w == 0 {
				panic("Name was not null terminated in index")
			}

			// Undo the last read, which wasn't whitespace..
			file.Seek(-1, 1)
		*/

	} else {
		panic("TODO: I can't handle such long names yet")
	}
	return &GitIndexEntry{f, string(name)}, nil
}

// Adds a file to the index, without writing it to disk.
// To write it to disk after calling this, use GitIndex.WriteIndex
//
// This will do the following:
// write git object blob of file contents to .git/objects
// normalize os.File name to path relative to gitRoot
// search GitIndex for normalized name
//	if GitIndexEntry found
//		update GitIndexEntry to point to the new object blob
// 	else
// 		add new GitIndexEntry if not found
//
func (g *GitIndex) AddFile(repo *libgit.Repository, file *os.File) {

	sha1, err := repo.StoreObjectLoose(libgit.ObjectBlob, file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error storing object: %s", err)
	}
	fmt.Printf("Sha1: %s\n", sha1)
	fmt.Printf("Name is %s\n", file.Name())
	name := getNormalizedName(file)
	for _, entry := range g.Objects {
		if entry.PathName == name {
			entry.Sha1 = sha1
			return
		}
	}
	panic("TODO: Unimplemented: add new file to GitIndexEntry")
}

func getNormalizedName(file *os.File) string {
	return file.Name()
}

// This will write a new index file to w by doing the following:
// 1. Sort the objects in g.Index to ascending order based on name
// 2. Write g.fixedGitIndex to w
// 3. for each entry in g.Objects, write it to w.
// 4. Write the Sha1 of the contents of what was written
func (g GitIndex) WriteIndex(file io.Writer) {
	sort.Sort(ByPath(g.Objects))
	s := sha1.New()
	w := io.MultiWriter(file, s)
	binary.Write(w, binary.BigEndian, g.fixedGitIndex)
	for _, entry := range g.Objects {
		binary.Write(w, binary.BigEndian, entry.fixedIndexEntry)
		binary.Write(w, binary.BigEndian, []byte(entry.PathName))
		padding := 8 - ((82 + len(entry.PathName) + 4) % 8)
		p := make([]byte, padding)
		binary.Write(w, binary.BigEndian, p)
	}
	binary.Write(w, binary.BigEndian, s.Sum(nil))

}

// Implement the sort interface on *GitIndexEntry, so that
// it's easy to sort by name.
type ByPath []*GitIndexEntry

func (g ByPath) Len() int           { return len(g) }
func (g ByPath) Swap(i, j int)      { g[i], g[j] = g[j], g[i] }
func (g ByPath) Less(i, j int) bool { return g[i].PathName < g[j].PathName }

func writeIndexSubtree(repo *libgit.Repository, prefix string, entries []*GitIndexEntry) ([20]byte, error) {
	content := bytes.NewBuffer(nil)
	// [mode] [file/folder name]\0[SHA-1 of referencing blob or tree as [20]byte]

	lastname := ""
	firstIdxForTree := -1

	for idx, obj := range entries {
		relativename := strings.TrimPrefix(obj.PathName, prefix+"/")
		//	fmt.Printf("This name: %s\n", relativename)
		nameBits := strings.Split(relativename, "/")

		// Either it's the last entry and we haven't written a tree yet, or it's not the last
		// entry but the directory changed
		if (nameBits[0] != lastname || idx == len(entries)-1) && lastname != "" {
			newPrefix := prefix + "/" + lastname

			var islice []*GitIndexEntry
			if idx == len(entries)-1 {
				islice = entries[firstIdxForTree:]
			} else {
				islice = entries[firstIdxForTree:idx]
			}
			subsha1, err := writeIndexSubtree(repo, newPrefix, islice)
			if err != nil {
				panic(err)
			}

			// Write the object
			fmt.Fprintf(content, "%o %s\x00", 0040000, lastname)
			content.Write(subsha1[:])

			if idx == len(entries)-1 && lastname != nameBits[0] {
				newPrefix := prefix + "/" + nameBits[0]
				subsha1, err := writeIndexSubtree(repo, newPrefix, entries[len(entries)-1:])
				if err != nil {
					panic(err)
				}

				// Write the object
				fmt.Fprintf(content, "%o %s\x00", 0040000, nameBits[0])
				content.Write(subsha1[:])

			}
			// Reset the data keeping track of what this tree is.
			lastname = ""
			firstIdxForTree = -1
		}
		if len(nameBits) == 1 {
			//write the blob for the file portion
			fmt.Fprintf(content, "%o %s\x00", obj.Mode, nameBits[0])
			content.Write(obj.Sha1[:])
			lastname = ""
			firstIdxForTree = -1
		} else {
			// calculate the sub-indexes to recurse on for this tree
			lastname = nameBits[0]
			if firstIdxForTree == -1 {
				firstIdxForTree = idx
			}
		}
	}

	sha1, err := repo.StoreObjectLoose(libgit.ObjectTree, bytes.NewReader(content.Bytes()))
	return sha1, err
}
func writeIndexEntries(repo *libgit.Repository, prefix string, entries []*GitIndexEntry) ([20]byte, error) {
	content := bytes.NewBuffer(nil)
	// [mode] [file/folder name]\0[SHA-1 of referencing blob or tree as [20]byte]

	lastname := ""
	firstIdxForTree := -1

	for idx, obj := range entries {
		nameBits := strings.Split(obj.PathName, "/")

		// Either it's the last entry and we haven't written a tree yet, or it's not the last
		// entry but the directory changed
		if (nameBits[0] != lastname || idx == len(entries)-1) && lastname != "" {
			var islice []*GitIndexEntry
			if idx == len(entries)-1 {
				islice = entries[firstIdxForTree:]
			} else {
				islice = entries[firstIdxForTree:idx]
			}
			subsha1, err := writeIndexSubtree(repo, lastname, islice)
			if err != nil {
				panic(err)
			}
			// Write the object
			fmt.Fprintf(content, "%o %s\x00", 0040000, lastname)
			content.Write(subsha1[:])

			// Reset the data keeping track of what this tree is.
			lastname = ""
			firstIdxForTree = -1
		}
		if len(nameBits) == 1 {
			//write the blob for the file portion
			fmt.Fprintf(content, "%o %s\x00", obj.Mode, obj.PathName)
			content.Write(obj.Sha1[:])
			lastname = ""
		} else {
			lastname = nameBits[0]
			if firstIdxForTree == -1 {
				firstIdxForTree = idx
			}
		}
	}

	sha1, err := repo.StoreObjectLoose(libgit.ObjectTree, bytes.NewReader(content.Bytes()))
	return [20]byte(sha1), err
}

// WriteTree writes the current index to a tree object.
// It returns the sha1 of the written tree, or an empty string
// if there was an error
func (g GitIndex) WriteTree(repo *libgit.Repository) string{

	sha1, err := writeIndexEntries(repo, "", g.Objects)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%x", sha1)
}

func expandGitTreeIntoIndexes(repo *libgit.Repository, tree *libgit.Tree, prefix string) ([]*GitIndexEntry, error) {
	newEntries := make([]*GitIndexEntry, 0)

	for _, entry := range tree.ListEntries() {
		var dirname string
		if prefix == "" {
			dirname = entry.Name()
		} else {
			dirname = prefix + "/" + entry.Name()
		}
		switch entry.Type {
		case libgit.ObjectBlob:
			newEntry := GitIndexEntry{}
			newEntry.Sha1 = entry.Id
			// This isn't right.
			// This should be:
			// "32-bit mode, split into:
			//      4-bit object type: valid values in binary are
			//          1000 (regular file), 1010 (symbolic link), and
			//          1110 (gitlink)
			//      3-bit unused
			//      9-bit unix permission. Only 0755 and 0644 are valid
			//          for regular files, symbolic links have 0 in this
			//          field"

			//go-gits entry mode is an int, but it needs to be a uint32
			switch entry.EntryMode() {
			case libgit.ModeBlob:
				newEntry.Mode = 0100644
			case libgit.ModeExec:
				newEntry.Mode = 0100755
			case libgit.ModeSymlink:
				newEntry.Mode = 0120000
			case libgit.ModeCommit:
				newEntry.Mode = 0160000
			case libgit.ModeTree:
				newEntry.Mode = 0040000
			}
			newEntry.PathName = dirname
			newEntry.Fsize = uint32(entry.Size())

			modTime := entry.ModTime()
			newEntry.Mtime = uint32(modTime.Unix())
			newEntry.Mtimenano = uint32(modTime.Nanosecond())
			newEntry.Flags = uint16(len(dirname)) & 0xFFF
			/* I don't know how git can extract these values
			   from a tree. For now, leave them empty

			   Ctime     uint32
			   Ctimenano uint32
			   Dev uint32
			   Ino uint32
			   Uid uint32
			   Gid uint32
			*/
			newEntries = append(newEntries, &newEntry)
		case libgit.ObjectTree:

			subTree, err := tree.SubTree(entry.Name())
			if err != nil {
				panic(err)
			}
			subindexes, err := expandGitTreeIntoIndexes(repo, subTree, dirname)
			if err != nil {
				panic(err)
			}
			newEntries = append(newEntries, subindexes...)

		default:
			fmt.Fprintf(os.Stderr, "Unknown object type: %s\n", entry.Type)
		}
	}
	return newEntries, nil

}
func (g *GitIndex) ResetIndex(repo *libgit.Repository, tree *libgit.Tree) error {
	newEntries, err := expandGitTreeIntoIndexes(repo, tree, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resetting index: %s\n", err)
		return err
	}
	fmt.Printf("%s", newEntries)
	g.NumberIndexEntries = uint32(len(newEntries))
	g.Objects = newEntries
	return nil
}
