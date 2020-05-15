package git

import (
	"bytes"
	"fmt"
	"io"
	// "io/ioutil"

	"compress/flate"
)

type deltaeval struct {
	value []byte
}

type delta struct {
	src  flate.Reader
	base io.ReadSeeker

	sz uint64
	// unread from the last read
	cached []byte
}

func newDelta(src flate.Reader, base io.ReadSeeker) delta {
	d := delta{src: src, base: base}
	// Read the source length. We don't care about the value,
	// but need to advance the stream by an approprite amount
	ReadVariable(src)

	// Read the target length so we know when we've finished processing
	// the delta stream.
	d.sz = ReadVariable(src)
	return d
}

func (d *delta) Read(buf []byte) (n int, err error) {
	// Read the cached data before we read more from
	// the stream
	if len(d.cached) > 0 {
		return d.readCached(buf)
	}
	instruction, err := d.src.ReadByte()
	if err != nil {
		return 0, err
	}
	if instruction >= 128 {
		var offset, length uint64

		if instruction&0x01 != 0 {
			o, err := d.src.ReadByte()
			if err != nil {
				return 0, err
			}
			offset = uint64(o)
		}
		if instruction&0x02 != 0 {
			o, err := d.src.ReadByte()
			if err != nil {
				return 0, err
			}
			offset |= uint64(o) << 8
		}
		if instruction&0x04 != 0 {
			o, err := d.src.ReadByte()
			if err != nil {
				return 0, err
			}
			offset |= uint64(o) << 16
		}
		if instruction&0x08 != 0 {
			o, err := d.src.ReadByte()
			if err != nil {
				return 0, err
			}
			offset |= uint64(o) << 24
		}

		if instruction&0x10 != 0 {
			l, err := d.src.ReadByte()
			if err != nil {
				return 0, err
			}
			length = uint64(l)
		}
		if instruction&0x20 != 0 {
			l, err := d.src.ReadByte()
			if err != nil {
				return 0, err
			}
			length |= uint64(l) << 8
		}
		if instruction&0x40 != 0 {
			l, err := d.src.ReadByte()
			if err != nil {
				return 0, err
			}
			length |= uint64(l) << 16
		}

		if length == 0 {
			length = 0x10000
		}

		if _, err := d.base.Seek(int64(offset), io.SeekStart); err != nil {
			return 0, err
		}
		if uint64(len(buf)) >= length {
			// It should all fit in the buffer
			n, err := io.ReadFull(d.base, buf[:length])
			if err != nil {
				return n, err
			}
			if int(n) != int(length) {
				return n, fmt.Errorf("Could not read %v bytes", length)
			}
			return n, err
		}
		// It's not going to all fit in the buffer, so cache it
		// and then return from the cache
		d.cached = make([]byte, length)
		if _, err := io.ReadFull(d.base, d.cached); err != nil {
			return 0, err
		}
		return d.readCached(buf)
	} else {
		// Read instruction bytes
		length := int(instruction)
		if len(buf) >= length {
			// It fits into the buffer, so don't
			// bother caching,
			n, err := io.ReadFull(d.src, buf[:length])
			if n != length {
				return n, fmt.Errorf("Could not read %v bytes", n)
			}
			return n, err
		}
		d.cached = make([]byte, length)
		if n, err := io.ReadFull(d.src, d.cached); err != nil {
			return n, err
		}
		if n != length {
			return n, fmt.Errorf("Insert: Could not read %v bytes", n)
		}
		return d.readCached(buf)
	}
	panic("Unreachable code reached")
}

func (d *delta) readCached(buf []byte) (n int, err error) {
	n = copy(buf, d.cached)
	if n == len(d.cached) {
		d.cached = nil
	} else {
		d.cached = d.cached[n:]
	}
	return n, nil
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
