package main

import (
	"encoding/binary"
	"errors"
	"fmt"
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

func ReadIndex() (*GitIndex, error) {
	file, err := os.Open(".git/index")
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
		whitespace := make([]byte, 1, 1)
		var w int
		// Read all the whitespace that git uses for alignment.
		for _, _ = file.Read(whitespace); whitespace[0] == 0; _, _ = file.Read(whitespace) {
			w += 1
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
