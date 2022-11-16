package delta

import (
	"fmt"
	"io"

	"compress/flate"
)

// Reader reads a Delta from Src and computes the
// delta using Base to resolve inserts.
type Reader struct {
	src  flate.Reader
	base io.ReadSeeker

	sz uint64
	// unread from the last read
	cached []byte
}

// Creates a new Delta using src and base as the source and
func NewReader(src flate.Reader, base io.ReadSeeker) Reader {
	d := Reader{src: src, base: base}
	// Read the source length. We don't care about the value,
	// but need to advance the stream by an approprite amount
	d.readVariable()

	// Read the target length so we know when we've finished processing
	// the delta stream.
	d.sz = d.readVariable()
	return d
}

// Reads the resolved delta from the underlying delta stream into buf
func (d *Reader) Read(buf []byte) (int, error) {
	// Read the cached data before we read more from
	// the stream
	if len(d.cached) > 0 {
		return d.readCached(buf)
	}
	instruction, err := d.src.ReadByte()
	if err != nil {
		return 0, err
	}
	if instruction == 0 {
		panic("Reserved/undocumented instruction found.")
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
		n, err := io.ReadFull(d.src, d.cached)
		if err != nil {
			return n, err
		}
		if n != length {
			return n, fmt.Errorf("Insert: Could not read %v bytes", length)
		}
		return d.readCached(buf)
	}
	panic("Unreachable code reached")
}

func (d *Reader) readCached(buf []byte) (n int, err error) {
	n = copy(buf, d.cached)
	if n == len(d.cached) {
		d.cached = nil
	} else {
		d.cached = d.cached[n:]
	}
	return n, nil
}

// Reads a variable length integer from the underlying stream and
// returns it as a uint64
func (d *Reader) readVariable() uint64 {
	var val uint64
	var i uint = 0
	for {
		b, err := d.src.ReadByte()
		if err != nil {
			panic(err)
		}
		val |= uint64(b&127) << (i * 7)
		if b < 128 {
			break
		}
		i += 1
	}
	return val
}

func (d Reader) Len() int {
	return int(d.sz)
}
