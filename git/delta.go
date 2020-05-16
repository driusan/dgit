package git

import (
	"fmt"
	"io"

	"compress/flate"
)

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
