package git

import (
	"bytes"
	"fmt"
	"io"
)

type deltaeval struct {
	value []byte
}

// Insert length bytes from src into d.
func (d *deltaeval) Insert(src io.Reader, length uint8) error {

	val := make([]byte, length)
	n, err := src.Read(val)
	if err != nil {
		return err
	}
	if n != int(length) {
		return fmt.Errorf("Could not insert %d byte of data. Got %d..", length, n)
	}
	d.value = append(d.value, val...)
	return nil
}

// Copies length bytes from src at offset offset
func (d *deltaeval) Copy(src []byte, offset, length uint64) error {
	if offset+length > uint64(len(src)) {
		return fmt.Errorf("Not enough data to copy.")
	}
	d.value = append(d.value, src[offset:offset+length]...)
	return nil
}

// Resolve a delta stream using resolvedDelta (a delta previous to this one
// in the pack file) as the base for Copy instructions.)
//
// This is largely based on the delta algorithm as described at:
// http://stefan.saasen.me/articles/git-clone-in-haskell-from-the-bottom-up/#pack_file_format
func calculateDelta(ref resolvedDelta, delta []byte) (PackEntryType, []byte, error) {
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
		if err := d.DoInstruction(deltaStream, ref.Value, targetLength); err != nil {
			return 0, nil, err
		}
		if targetLength == uint64(len(d.value)) {
			break
		}
		if len(d.value) > int(targetLength) {
			panic("Read too much data from delta stream")
		}
	}
	return ref.Type, d.value, nil

}

// Calculate an offset delta. refs must be a map of all previous references in
// the packfile.
func calculateOfsDelta(ref ObjectOffset, delta []byte, refs map[ObjectOffset]resolvedDelta) (PackEntryType, []byte, error) {
	refdata, ok := refs[ref]
	if !ok {
		return 0, nil, fmt.Errorf("Thin packs are not currently supported.")
	}
	return calculateDelta(refdata, delta)
}

// A Delta base which has already been resolved earlier in the pack file.
type resolvedDelta struct {
	Value []byte
	Type  PackEntryType
}

// Performs a delta instruction (either add or copy) from delta reader, using
// ref as a reference to copy from. Target size is the expected final size.
func (d *deltaeval) DoInstruction(delta io.Reader, src []byte, targetSize uint64) error {
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

		return d.Copy(src, uint64(offset), length)
	} else {
		return d.Insert(delta, uint8(b[0]))
	}
}

// Calculate a reference delta. refs must be a map of all previous objects in
// the packfile.
func calculateRefDelta(ref Sha1, delta []byte, refs map[Sha1]resolvedDelta) (PackEntryType, []byte, error) {
	refdata, ok := refs[ref]
	if !ok {
		return 0, nil, fmt.Errorf("Thin packs are not currently supported.")
	}
	return calculateDelta(refdata, delta)
}
