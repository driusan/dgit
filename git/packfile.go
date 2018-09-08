package git

import (
	"bytes"
	"fmt"
	"io"

	"github.com/driusan/dgit/zlib"
)

var debug bool = false

type PackfileHeader struct {
	Signature [4]byte
	Version   uint32
	Size      uint32
}

type PackEntryType uint8
type PackEntrySize uint64
type ObjectReference []byte
type ObjectOffset int

const (
	OBJ_COMMIT    PackEntryType = 1
	OBJ_TREE      PackEntryType = 2
	OBJ_BLOB      PackEntryType = 3
	OBJ_TAG       PackEntryType = 4
	_             PackEntryType = 5 // reserved for future use
	OBJ_OFS_DELTA PackEntryType = 6
	OBJ_REF_DELTA PackEntryType = 7
)

func (t PackEntryType) String() string {
	switch t {
	case OBJ_COMMIT:
		return "commit"
	case OBJ_TREE:
		return "tree"
	case OBJ_BLOB:
		return "blob"
	case OBJ_TAG:
		return "tag"
	case OBJ_OFS_DELTA:
		return "ofs_delta"
	case OBJ_REF_DELTA:
		return "ref_delta"
	default:
		return "unknown"
	}
}

// Readers a packfile entry header from r, and returns the type of packfile,
// the size from the header, optionally a reference or file offset (for deltas
// only), and any data read from the io stream.
func (p PackfileHeader) ReadHeaderSize(r io.Reader) (PackEntryType, PackEntrySize, Sha1, ObjectOffset, []byte) {
	b := make([]byte, 1)
	var i uint
	var size PackEntrySize
	var entrytype PackEntryType
	refDelta := make([]byte, 20)

	// allocate a little bit of space, to go easier on the GC. We don't know
	// exactly how much will be read because the size is variable, but most
	// headers should be less than 32 bytes.
	dataread := make([]byte, 0, 32)
	for {
		if _, err := r.Read(b); err != nil {
			panic(err)
		}
		dataread = append(dataread, b...)
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
	switch entrytype {
	case OBJ_REF_DELTA:
		n, err := r.Read(refDelta)
		if n != 20 || err != nil {
			panic(err)
		}
		dataread = append(dataread, refDelta...)
		sha, err := Sha1FromSlice(refDelta)
		if err != nil {
			panic(err)
		}
		return entrytype, size, sha, 0, dataread
	case OBJ_OFS_DELTA:
		deltaOffset, raw := ReadDeltaOffset(r)
		dataread = append(dataread, raw...)
		return entrytype, size, Sha1{}, ObjectOffset(deltaOffset), dataread
	}
	return entrytype, size, Sha1{}, 0, dataread
}

func (p PackfileHeader) ReadEntryDataStream(r io.ReadSeeker) (uncompressed []byte, compressed []byte) {
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

type VariableLengthInt uint64

func (v VariableLengthInt) WriteVariable(w io.Writer, typ PackEntryType) error {
	b := make([]byte, 0)
	// Encode the type
	theByte := byte(typ) << 4
	// Encode the last 4 bits
	theByte |= byte(v & 0xF)
	v = v >> 4
	b = append(b, theByte)
	for {
		if v == 0 {
			break
		}

		b[len(b)-1] |= 0x80

		theByte = byte(v & 0x7F)
		b = append(b, theByte)
		v = v >> 7

	}
	w.Write(b)
	return nil
}

// Reads a delta offset from the io.Reader, and returns both the value
// and the list of bytes consumed from the reader.
func ReadDeltaOffset(src io.Reader) (uint64, []byte) {
	b := make([]byte, 1)
	consumed := make([]byte, 0, 32)
	var val uint64
	src.Read(b)
	consumed = append(consumed, b...)
	val = uint64(b[0] & 127)
	for i := 0; b[0]&128 != 0; i++ {
		val += 1
		if debug {
			fmt.Printf("%x ", b)
		}
		src.Read(b)
		consumed = append(consumed, b...)
		val = (val << 7) + uint64(b[0]&127)
	}
	return val, consumed
}
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

func writeResolvedObject(c *Client, t PackEntryType, rawdata []byte) (Sha1, error) {
	switch t {
	case OBJ_COMMIT, OBJ_TREE, OBJ_BLOB:
		// Do nothing. We're just checking that it's a type we can
		// handle.
	default:
		return Sha1{}, fmt.Errorf("Unknown type: %s", t)
	}
	sha, err := c.WriteObject(t.String(), rawdata)
	if err != nil {
		return sha, err
	}
	return sha, nil
}
