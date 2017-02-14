package git

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	libgit "github.com/driusan/git"
	"io"
)

func SendPackfile(c *Client, w io.Writer, objects []Sha1) error {
	repo, err := libgit.OpenRepository(c.GitDir.String())
	if err != nil {
		return err
	}

	sha := sha1.New()
	w = io.MultiWriter(w, sha)
	n, err := w.Write([]byte{'P', 'A', 'C', 'K'})
	if n != 4 {
		panic("Could not write signature")
	}
	if err != nil {
		return err
	}

	// Version
	binary.Write(w, binary.BigEndian, uint32(2))
	// Size
	binary.Write(w, binary.BigEndian, uint32(len(objects)))
	for _, obj := range objects {
		s := VariableLengthInt(obj.UncompressedSize(repo))
		err := s.WriteVariable(w, obj.PackEntryType(repo))
		if err != nil {
			return err
		}

		err = obj.CompressedWriter(repo, w)
		if err != nil {
			return err
		}
	}
	trailer := sha.Sum(nil)
	w.Write(trailer)
	return nil
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

type resolvedDelta struct {
	Value []byte
	Type  PackEntryType
}

var deltaChains map[ObjectOffset]resolvedDelta

func (d *deltaeval) CopyOfs(repo *libgit.Repository, p PackfileHeader, src io.ReadSeeker, offset int64, length uint64) {
	bookmark, _ := src.Seek(0, io.SeekCurrent)
	t, _, _, foffset := p.ReadHeaderSize(src)

	var r io.Reader
	var err error
	switch t {
	case OBJ_COMMIT, OBJ_TREE, OBJ_BLOB, OBJ_TAG:
		// the ReadHeaderSize got past the object header.
		// Next is the zlib stream, but we put it into a temporary
		// variable and then into r so that we can defer Close() it
		zr, err := zlib.NewReader(src)
		if err != nil {
			panic(err)
		}
		defer zr.Close()
		r = zr
	case OBJ_OFS_DELTA:
		// If this offset is pointing to an offset delta, we should have
		// already resolved it before getting here, similarly to how
		// OBJ_REF_DELTAs only refer to things that occured earlier
		// in the packfile.
		rawdata := deltaChains[ObjectOffset(bookmark)-ObjectOffset(foffset)]
		r = bytes.NewReader(rawdata.Value)
	}

	if offset > 0 {
		tmp := make([]byte, offset)
		n, err := io.ReadFull(r, tmp)
		if err != nil && err != io.EOF {
			panic(err)
		}
		if n == 0 || int64(n) != offset {
			panic(fmt.Sprintf("Couldn't correctly read offset %d != %d.", n, offset))
		}

	}

	reading := make([]byte, length)
	n, err := io.ReadFull(r, reading)
	if err != nil && err != io.EOF {
		panic(err)
	}
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

func (d *deltaeval) DoOfsInstruction(repo *libgit.Repository, p PackfileHeader, delta io.Reader, src io.ReadSeeker, absref ObjectOffset, targetSize uint64) {
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

		// Go to the reference for the copy.
		src.Seek(int64(absref), io.SeekStart)
		d.CopyOfs(repo, p, src, int64(offset), length)
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
	panic(fmt.Sprintf("Unhandle object type while calculating delta: %v", objt))
	return 0, nil
}

// Modified version of the above which uses file offsets instead of reference
// offsets.
func calculateOfsDelta(repo *libgit.Repository, p PackfileHeader, r io.ReadSeeker, reference ObjectOffset, delta []byte) (PackEntryType, []byte) {
	deltaStream := bytes.NewBuffer(delta)

	// Restore the pointer for r after the delta is calculated.
	bookmark, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		panic(err)
	}

	defer func(i int64) {
		r.Seek(i, io.SeekStart)
	}(bookmark)

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
		d.DoOfsInstruction(repo, p, deltaStream, r, reference, targetLength)
		if targetLength == uint64(len(d.value)) {
			break
		}
		if len(d.value) > int(targetLength) {
			panic("Read too much data from delta stream")
		}
	}
	r.Seek(int64(reference), io.SeekStart)

	objt, _, _, foffset := p.ReadHeaderSize(r)

	switch objt {
	case OBJ_COMMIT, OBJ_TREE, OBJ_BLOB, OBJ_TAG:
		return objt, d.value
	case OBJ_OFS_DELTA:
		val := deltaChains[reference - ObjectOffset(foffset)]
		// Use the type of the fully resolved reference from our chain
		// map, and the value we just calculated.
		return val.Type, d.value
	default:
		panic(fmt.Sprintf("Unhandle object type while calculating delta: %v", objt))
	}
}

func writeResolvedObject(c *Client, t PackEntryType, rawdata []byte) (Sha1, error) {
	var sType string
	switch t {
	case OBJ_COMMIT:
		sType = "commit"
	case OBJ_TREE:
		sType = "tree"
	case OBJ_BLOB:
		sType = "blob"
	default:
		return Sha1{}, fmt.Errorf("Unknown type: %s", t)
	}
	return c.WriteObject(sType, rawdata)
}

// Unpacks a packfile into the GitDir of c and returns the Sha1
// of everything that was unpacked.
func Unpack(c *Client, r io.ReadSeeker) ([]Sha1, error) {
	repo, err := libgit.OpenRepository(c.GitDir.String())
	if err != nil {
		return nil, err
	}

	var p PackfileHeader
	binary.Read(r, binary.BigEndian, &p)
	if p.Signature != [4]byte{'P', 'A', 'C', 'K'} {
		return nil, fmt.Errorf("Invalid packfile.")
	}
	if p.Version != 2 {
		return nil, fmt.Errorf("Unsupported packfile version: %d", p.Version)
	}

	deltaChains = make(map[ObjectOffset]resolvedDelta)

	var objects []Sha1
	for i := uint32(0); i < p.Size; i += 1 {
		start, err := r.Seek(0, io.SeekCurrent)
		if err != nil {
			return objects, err
		}
		t, s, ref, offset := p.ReadHeaderSize(r)
		rawdata, _ := p.ReadEntryDataStream(r)
		switch t {
		case OBJ_COMMIT, OBJ_TREE, OBJ_BLOB:
			sha1, err := writeResolvedObject(c, t, rawdata)
			if err != nil {
				return objects, err
			}
			objects = append(objects, sha1)
		case OBJ_OFS_DELTA:
			t, deltadata := calculateOfsDelta(repo, p, r, ObjectOffset(start)-offset, rawdata)
			deltaChains[ObjectOffset(start)-offset] = resolvedDelta{deltadata, t}
			switch t {
			case OBJ_COMMIT, OBJ_TREE, OBJ_BLOB:
				sha1, err := writeResolvedObject(c, t, deltadata)
				if err != nil {
					return objects, err
				}
				objects = append(objects, sha1)
			default:
				panic(fmt.Sprintf("TODO: Unhandled type", t))

			}
		case OBJ_REF_DELTA:
			t, deltadata := calculateDelta(repo, ref, rawdata)
			switch t {
			case OBJ_COMMIT, OBJ_TREE, OBJ_BLOB:
				sha1, err := writeResolvedObject(c, t, deltadata)
				if err != nil {
					return objects, err
				}
				objects = append(objects, sha1)
			default:
				panic(fmt.Sprintf("TODO: Unhandled type", t))

			}
		default:
			panic(fmt.Sprintf("TODO: Unhandled type", t))
		}

		if len(rawdata) != int(s) {
			panic(fmt.Sprintf("Incorrect size of entry %d: %d not %d", i, len(rawdata), s))
		}
	}
	return objects, nil
}
