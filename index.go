package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	libgit "github.com/gogits/git"
	//"io"
	"os"
)

var InvalidIndex error = errors.New("Invalid index")

// index file is defined as Network byte Order (Big Endian)

// 12 byte header:
// 4 bytes: D I R C (stands for "Dir cache")
// 4-byte version number (can be 2, 3 or 4)
// 32bit number of index entries
type fixedGitIndex struct {
	Signature          [4]byte
	Version            uint32
	NumberIndexEntries uint32
}

type GitIndex struct {
	fixedGitIndex
	Objects []*GitIndexEntry
}

type fixedIndexEntry struct {
	Ctime     uint32
	Ctimenano uint32

	Mtime     uint32
	Mtimenano uint32

	Dev uint32
	Ino uint32

	Mode uint32

	Uid uint32
	Gid uint32

	Fsize uint32

	Sha1 [20]byte

	Flags uint16
}

type GitIndexEntry struct {
	fixedIndexEntry

	PathName string
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
