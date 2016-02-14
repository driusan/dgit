package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
	libgit "github.com/driusan/git"
	"github.com/driusan/go-git/zlib"
	"io"
	"os"
)

var debug bool = false
var ObjectExists = errors.New("Object already exists")

type packfile struct {
	Signature [4]byte
	Version   uint32
	Size      uint32
}

type PackEntryType uint8
type PackEntrySize uint64
type ObjectReference []byte

const (
	OBJ_COMMIT    PackEntryType = 1
	OBJ_TREE      PackEntryType = 2
	OBJ_BLOB      PackEntryType = 3
	OBJ_TAG       PackEntryType = 4
	_             PackEntryType = 5 // reserved for future use
	OBJ_OFS_DELTA PackEntryType = 6
	OBJ_REF_DELTA PackEntryType = 7
)

func (p packfile) ReadHeaderSize(r io.Reader) (PackEntryType, PackEntrySize, ObjectReference) {
	b := make([]byte, 1)
	var i uint
	var size PackEntrySize
	var entrytype PackEntryType
	refDelta := make([]byte, 20)
	for {
		r.Read(b)
		if i == 0 {
			// Extract bits 2-4, which contain the type
			// 0x70 is the bitmask for bits 2-4, then shift
			// it over 4 bits so that it fits into the entry
			// type uint8 and can be compared against the
			// consts
			switch entrytype = PackEntryType((b[0] & 0x70) >> 4); entrytype {
			case OBJ_COMMIT:
			case OBJ_TREE:
			case OBJ_BLOB:
			case OBJ_TAG:
				fmt.Printf("Tag!\n")
			case OBJ_OFS_DELTA:
				fmt.Printf("OFS_DELTA!\n")
			case OBJ_REF_DELTA:

			}
			// on the first byte, bits 5-8 are the size
			size = PackEntrySize(b[0] & 0x0F)
		} else {
			// on anything after the first byte, bit 1
			// tells us if this is the last byte in the
			// header, and bits 2-8 are part of the
			// size.
			// b[0] & 0x7F extracts those bits, but then
			// they need to be shifted by a constant 4 bits
			// (because those 4 bits were from the first byte),
			// plus 7 for each of the previous bytes to get
			// the location for bits 2-8 from this byte.
			var tmp uint64 = uint64(b[0] & 0x7f)
			size |= PackEntrySize(tmp << (4 + ((i - 1) * 7)))
		}
		if b[0] < 128 {
			break
		}
		i += 1
	}
	if entrytype == OBJ_REF_DELTA {
		n, err := r.Read(refDelta)
		if n != 20 || err != nil {
			panic(err)
		}
	}
	return entrytype, size, refDelta
}

// Returns
func (p packfile) ReadEntryDataStream(r io.ReadSeeker) (uncompressed []byte, compressed []byte) {
	b := new(bytes.Buffer)
	bookmark, _ := r.Seek(0, 1)
	zr, err := zlib.NewReader(r)
	if err != nil {
		panic(err)
	}
	defer zr.Close()
	io.Copy(b, zr)

	// Go's zlib implementation is greedy, so we need some hacks to
	// get r back to the right place in the file.
	// We use a modified version of compress/zlib which exposes the
	// digest. Before reading, we take a bookmark of the address
	// that we're starting at, then after reading we go back there.
	// Starting from there, look through the reader until we find the
	// compressed object's zlib digest.
	// This is stupid, but necessary because git's packfile format
	// is *very* stupid.
	digest := zr.Digest.Sum32()
	r.Seek(bookmark, 0)
	address := make([]byte, 4)
	var i int64
	var finalAddress int64
	for {
		n, err := r.Read(address)
		// This probably means we reached the end of the io.Reader.
		// It might be the last read, so break out of the loop instead
		// of getting caught in an infinite loop.
		if n < 4 || err != nil {
			break
		}
		var intAddress uint32 = (uint32(address[3]) | uint32(address[2])<<8 | uint32(address[1])<<16 | uint32(address[0])<<24)
		if intAddress == digest {
			finalAddress = bookmark + i + 4
			break
		}
		// Advance a byte
		i += 1
		r.Seek(bookmark+i, 0)

	}
	r.Seek(bookmark, 0)
	compressed = make([]byte, finalAddress-bookmark)
	r.Read(compressed)
	r.Seek(finalAddress, 0)
	return b.Bytes(), compressed

}

func writeObject(repo *libgit.Repository, objType string, rawdata []byte) error {
	obj := []byte(fmt.Sprintf("%s %d\000", objType, len(rawdata)))
	obj = append(obj, rawdata...)
	sha := sha1.Sum(obj)

	if have, _, err := repo.HaveObject(fmt.Sprintf("%x", sha)); have == true || err != nil {
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			return err

		}

		//fmt.Fprintf(os.Stderr, "Already have object %x\n", sha)
		return ObjectExists

	}
	directory := fmt.Sprintf("%x", sha[0:1])
	file := fmt.Sprintf("%x", sha[1:])

	fmt.Printf("Putting %x in %s/%s\n", sha, directory, file)
	os.MkdirAll(repo.Path+"/objects/"+directory, os.FileMode(0755))
	f, err := os.Create(repo.Path + "/objects/" + directory + "/" + file)
	if err != nil {
		return err
	}
	defer f.Close()
	w := zlib.NewWriter(f)
	if _, err := w.Write(obj); err != nil {
		return err
	}
	defer w.Close()
	return nil
}

type VariableLengthInt uint64

func ReadVariable(src io.Reader) uint64 {
	b := make([]byte, 1)
	var val uint64
	var i uint = 0
	for {
		src.Read(b)
		val |= uint64(b[0]&127) << (i * 7)
		if b[0] < 128 {
			break
		}
		i += 1
	}
	return val
}

type deltaeval struct {
	value []byte
}

func (d *deltaeval) Insert(src io.Reader, length uint8) {

	val := make([]byte, length)
	n, err := src.Read(val)
	if err != nil || n != int(length) {
		panic(fmt.Sprintf("Couldn't read %d bytes: %s", length, err))
	}
	d.value = append(d.value, val...)
}
func (d *deltaeval) Copy(repo *libgit.Repository, src ObjectReference, offset, length uint64) {
	id, _ := libgit.NewId(src)
	_, _, r, err := repo.GetRawObject(id, false)
	if err != nil {
		panic(err)
	}
	defer r.Close()
	if offset > 0 {
		tmp := make([]byte, offset)
		n, err := io.ReadFull(r, tmp)
		if err != nil {
			panic(err)
		}
		if n == 0 || uint64(n) != offset {
			panic("Couldn't correctly read offset.")
		}

	}

	reading := make([]byte, length)
	n, err := io.ReadFull(r, reading)
	if uint64(n) != length || err != nil {
		panic("Error copying data")
	}
	d.value = append(d.value, reading...)
}
func (d *deltaeval) DoInstruction(repo *libgit.Repository, delta io.Reader, ref ObjectReference, targetSize uint64) {
	b := make([]byte, 1)

	delta.Read(b)
	if b[0] == 0 {
		panic("Unexpected delta opcode: 0")
	}
	if b[0] >= 128 {
		var offset, length uint64
		offset = 0
		length = 0
		lenBuf := make([]byte, 1)
		if b[0]&0x01 != 0 {
			delta.Read(lenBuf)
			offset = uint64(lenBuf[0])
		}
		if b[0]&0x02 != 0 {
			delta.Read(lenBuf)
			offset |= uint64(lenBuf[0]) << 8
		}
		if b[0]&0x04 != 0 {
			delta.Read(lenBuf)
			offset |= uint64(lenBuf[0]) << 16
		}
		if b[0]&0x08 != 0 {
			delta.Read(lenBuf)
			offset |= uint64(lenBuf[0]) << 24
		}

		if b[0]&0x10 != 0 {
			delta.Read(lenBuf)
			length = uint64(lenBuf[0])
		}
		if b[0]&0x20 != 0 {
			delta.Read(lenBuf)
			length |= uint64(lenBuf[0]) << 8
		}
		if b[0]&0x40 != 0 {
			delta.Read(lenBuf)
			length |= uint64(lenBuf[0]) << 16
		}

		if length == 0 {
			length = 0x10000
		}
		if length > targetSize {
			panic("Trying to read too much data.")
		}

		d.Copy(repo, ref, uint64(offset), length)
	} else {
		d.Insert(delta, uint8(b[0]))
	}
}

// Calculate the final reslt of a REF_DELTA. This is largely
// based on the delta algorithm as described at:
// http://stefan.saasen.me/articles/git-clone-in-haskell-from-the-bottom-up/#pack_file_format
func calculateDelta(repo *libgit.Repository, reference ObjectReference, delta []byte) (PackEntryType, []byte) {
	deltaStream := bytes.NewBuffer(delta)

	// Read 2 variable length strings for the source and target buffer
	// length

	// read the source length, but we don't care. We just want to advance
	// the deltaStream pointer the proper amount.
	ReadVariable(deltaStream)
	// Read the target length so we know when we've finished processing
	// the delta stream.
	targetLength := ReadVariable(deltaStream)

	d := deltaeval{}
	for {
		d.DoInstruction(repo, deltaStream, reference, targetLength)
		if targetLength == uint64(len(d.value)) {
			break
		}
		if len(d.value) > int(targetLength) {
			panic("Read too much data from delta stream")
		}
	}
	// GetRawObject to find the underlying type of the original
	// reference
	id, err := libgit.NewId(reference)
	if err != nil {
		panic(err)
	}
	objt, _, _, err := repo.GetRawObject(id, true)
	if err != nil {
		panic(err)
	}
	switch objt {
	case libgit.ObjectCommit:
		return OBJ_COMMIT, d.value
	case libgit.ObjectTree:
		return OBJ_TREE, d.value
	case libgit.ObjectBlob:
		return OBJ_BLOB, d.value
	case libgit.ObjectTag:
		return OBJ_TAG, d.value
	}
	panic("Unhandle object type while calculating delta.")
	return 0, nil

}
func unpack(repo *libgit.Repository, r io.ReadSeeker) {
	var p packfile
	binary.Read(r, binary.BigEndian, &p)
	if p.Signature != [4]byte{'P', 'A', 'C', 'K'} {
		return //err
	}
	if p.Version != 2 {
		return //err
	}
	for i := uint32(0); i < p.Size; i += 1 {
		t, s, ref := p.ReadHeaderSize(r)
		rawdata, _ := p.ReadEntryDataStream(r)
		switch t {
		case OBJ_COMMIT:
			writeObject(repo, "commit", rawdata)
		case OBJ_TREE:
			writeObject(repo, "tree", rawdata)
		case OBJ_BLOB:
			writeObject(repo, "blob", rawdata)
		case OBJ_REF_DELTA:
			t, deltadata := calculateDelta(repo, ref, rawdata)
			switch t {
			case OBJ_COMMIT:
				writeObject(repo, "commit", deltadata)
			case OBJ_TREE:
				writeObject(repo, "tree", deltadata)
			case OBJ_BLOB:
				writeObject(repo, "blob", deltadata)
			default:
				panic("TODO: Unhandled type for REF_DELTA")
			}
		default:
			panic(fmt.Sprintf("TODO: Unhandled type", t))
		}

		if len(rawdata) != int(s) {
			panic(fmt.Sprintf("Incorrect size of entry %d: %d not %d", i, len(rawdata), s))
		}
	}
}
