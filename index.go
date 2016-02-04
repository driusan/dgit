package main

import (
	"encoding/binary"
	"errors"
	"sort"
	"fmt"
	libgit "github.com/gogits/git"
	"io"
	//"io/ioutil"
	"crypto/sha1"
	"os"
)

var InvalidIndex error = errors.New("Invalid index")

// index file is defined as Network byte Order (Big Endian)

// 12 byte header:
// 4 bytes: D I R C (stands for "Dir cache")
// 4-byte version number (can be 2, 3 or 4)
// 32bit number of index entries
type fixedGitIndex struct {
	Signature          [4]byte // 4
	Version            uint32 // 8
	NumberIndexEntries uint32 // 12
}

type GitIndex struct {
	fixedGitIndex // 12
	Objects []*GitIndexEntry
}
type GitIndexEntry struct {
	fixedIndexEntry

	PathName string
}

type fixedIndexEntry struct {
	Ctime     uint32       // 16
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
		return nil, err
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
		whitespace := make([]byte, 1, 1)
		var w uint16
		// Read all the whitespace that git uses for alignment.
		for _, _ = file.Read(whitespace); whitespace[0] == 0; _, _ = file.Read(whitespace) {
			w += 1
		}

		if w != expectedOffset {
			panic("Read incorrect number of whitespace characters")
		}
		if w == 0 {
			panic("Name was not null terminated in index")
		}
		// Undo the last read, which wasn't whitespace..
		file.Seek(-1, 1)

	} else {
		panic("I can't handle such long names yet")
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
	panic("Unimplemented: add new file to GitIndexEntry")
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
		padding  := 8 - ((82 + len(entry.PathName) + 4) % 8)
		p := make([]byte, padding)
		binary.Write(w, binary.BigEndian, p)
	}
	binary.Write(w, binary.BigEndian, s.Sum(nil))
	
}

// Implement the sort interface on *GitIndexEntry, so that
// it's easy to sort by name.
type ByPath []*GitIndexEntry
func (g ByPath) Len() int { return len(g) }
func (g ByPath)  Swap(i, j int) { g[i], g[j] = g[j], g[i] }
func (g ByPath) Less(i, j int) bool { return g[i].PathName < g[j].PathName }

