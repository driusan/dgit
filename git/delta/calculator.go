package delta

import (
	"bytes"
	"container/list"
	"encoding/binary"
	"fmt"
	"index/suffixarray"
	"io"
)

// The minimum number of characters to copy from the stream. If
// there is not a prefix amount to copy from the stream.
const minCopy = 3

// We use a simple interface to make our calculate function easily
// testable and debuggable.
type instruction interface {
	// Write the instruction to w
	write(w io.Writer) error

	// Used by the test suite
	equals(i2 instruction) bool
}

// insert instruction. Insert the bytes into the stream.
type insert []byte

func (i insert) write(w io.Writer) error {
	remaining := []byte(i)
	for len(remaining) > 0 {
		if len(remaining) < 128 {
			// What's left fits in a single insert
			// instruction
			if _, err := w.Write([]byte{byte(len(remaining))}); err != nil {
				return err
			}
			if _, err := w.Write(remaining); err != nil {
				return err
			}
			remaining = nil
		} else {
			// What's left doesn't fit in a single
			// insert instruction, so insert the largest
			// amount that does
			if _, err := w.Write([]byte{127}); err != nil {
				return err
			}
			if _, err := w.Write(remaining[:127]); err != nil {
				return err
			}
			remaining = remaining[127:]
		}
	}
	return nil
}

func (i insert) equals(i2 instruction) bool {
	i2i, ok := i2.(insert)
	if !ok {
		return false
	}
	return string(i) == string(i2i)
}

type copyinst struct {
	offset, length uint32
}

func (c copyinst) equals(i2 instruction) bool {
	i2c, ok := i2.(copyinst)
	if !ok {
		return false
	}
	return i2c.offset == c.offset && i2c.length == c.length
}

// The meat of our algorithm. Calculate a list of instructions to
// insert into the stream.
func calculate(index *suffixarray.Index, src, dst []byte, maxsz int) (*list.List, error) {
	instructions := list.New()
	remaining := dst
	estsz := 0
	for len(remaining) > 0 {
		nexto, nextl := longestPrefix(index, remaining)
		if maxsz > 0 && estsz > maxsz {
			return nil, fmt.Errorf("Max size exceeded")
		}
		if nextl > 0 {
			estsz += 9
			instructions.PushBack(copyinst{uint32(nexto), uint32(nextl)})
			remaining = remaining[nextl:]
			continue
		}
		// FIXME: Find where the next prefix > minCopy starts,
		// insert until then instead of always inserting minCopy
		if len(remaining) <= minCopy {
			estsz += len(remaining) + 1
			instructions.PushBack(insert(remaining))
			remaining = nil
			continue
		}

		nextOffset := nextPrefixStart(index, dst)
		if nextOffset >= 0 {
			estsz += 1 + len(remaining) - nextOffset
			instructions.PushBack(insert(remaining[:nextOffset]))
			remaining = remaining[nextOffset:]
		} else {
			// nextPrefixStart went through the whole string
			// and didn't find anything, so insert the whole string
			estsz += len(remaining) + 1
			instructions.PushBack(insert(remaining))
			remaining = nil
		}

	}
	return instructions, nil
}

// Returns the longest prefix of dst that is found somewhere in src.
func longestPrefix(src *suffixarray.Index, dst []byte) (offset, length int) {
	// First the simple edge simple cases. Is it smaller than minCopy? Does
	// it have a prefix of at least minCopy?
	if len(dst) < minCopy {
		return 0, -1
	}

	// If there's no prefix at all of at least length minCopy,
	// don't bother searching for one.
	if result := src.Lookup(dst[:minCopy], 1); len(result) == 0 {
		return 0, -1
	}

	// If the entire dst exists somewhere in src, return the first
	// one found.
	if result := src.Lookup(dst, 1); len(result) > 0 {
		return result[0], len(dst)
	}

	// We know there's a substring somewhere but the whole thing
	// isn't a substring, brute force the location of the longest
	// substring with a binary search of our suffix array.
	length = -1
	minIdx := minCopy
	maxIdx := len(dst)
	for i := minIdx; maxIdx-minIdx > 1; i = ((maxIdx - minIdx) / 2) + minIdx {
		if result := src.Lookup(dst[:i], 1); result != nil {
			offset = result[0]
			length = i
			minIdx = i
		} else {
			maxIdx = i - 1
		}
	}
	return
}

// Find the start of the next prefix of dst that has a size of at least
// minCopy
func nextPrefixStart(src *suffixarray.Index, dst []byte) (offset int) {
	for i := 1; i < len(dst); i++ {
		end := i + minCopy
		if end > len(dst) {
			end = len(dst)
		}
		if result := src.Lookup(dst[i:end], 1); result != nil {
			return i
		}
	}
	return -1
}

func CalculateWithIndex(index *suffixarray.Index, w io.Writer, src, dst []byte, maxsz int) error {
	instructions, err := calculate(index, src, dst, maxsz)
	if err != nil {
		return err
	}
	// Write src and dst length header
	if err := writeVarInt(w, len(src)); err != nil {
		return err
	}
	if err := writeVarInt(w, len(dst)); err != nil {
		return err
	}
	// Write the instructions themselves
	for e := instructions.Front(); e != nil; e = e.Next() {
		inst := e.Value.(instruction)

		if err := inst.write(w); err != nil {
			return err
		}
	}
	return nil
}

// Calculate how to generate dst using src as the base
// of the deltas and write the result to w.
func Calculate(w io.Writer, src, dst []byte, maxsz int) error {
	index := suffixarray.New(src)
	return CalculateWithIndex(index, w, src, dst, maxsz)
}

func (c copyinst) write(w io.Writer) error {
	var buf bytes.Buffer
	instbyte := byte(0x80)

	// Set the offset bits in the instruction
	if c.offset&0xff != 0 {
		instbyte |= 0x01
	}
	if c.offset&0xff00 != 0 {
		instbyte |= 0x02
	}
	if c.offset&0xff0000 != 0 {
		instbyte |= 0x04
	}
	if c.offset&0xff000000 != 0 {
		instbyte |= 0x08
	}

	// Set the length bits in the instruction
	if c.length > 0xffffff {
		// FIXME: Decompose this into multiple copy
		// instructions
	} else if c.length == 0x10000 {
		// 0x10000 is a special case, encoded as 0
	} else {
		// Encode the bits in the byte that denote
		// which bits are incoming in the stream
		// for length
		if c.length&0xff != 0 {
			instbyte |= 0x10
		}

		if c.length&0xff00 != 0 {
			instbyte |= 0x20
		}

		if c.length&0xff0000 != 0 {
			instbyte |= 0x40
		}
	}
	// Write the header
	if err := buf.WriteByte(instbyte); err != nil {
		return err
	}

	// Write the offset bytes
	if val := byte(c.offset & 0xff); val != 0 {
		if err := buf.WriteByte(val); err != nil {
			return err
		}
	}
	if val := byte(c.offset >> 8 & 0xff); val != 0 {
		if err := buf.WriteByte(val); err != nil {
			return err
		}
	}
	if val := byte(c.offset >> 16 & 0xff); val != 0 {
		if err := buf.WriteByte(val); err != nil {
			return err
		}
	}
	if val := byte(c.offset >> 24 & 0xff); val != 0 {
		if err := buf.WriteByte(val); err != nil {
			return err
		}
	}

	// Write the length
	if c.length != 0x10000 {
		if val := byte(c.length & 0xff); val != 0 {
			if err := buf.WriteByte(val); err != nil {
				return err
			}
		}
		if val := byte((c.length >> 8) & 0xff); val != 0 {
			if err := buf.WriteByte(val); err != nil {
				return err
			}

		}
		if val := byte((c.length >> 16) & 0xff); val != 0 {
			if err := buf.WriteByte(val); err != nil {
				return err
			}

		}

	}
	if n, err := w.Write(buf.Bytes()); err != nil {
		return err
	} else if n != buf.Len() {
		return fmt.Errorf("Could not write entire instruction")
	}
	return nil
}

func writeVarInt(w io.Writer, val int) error {
	var buf [128]byte
	n := binary.PutUvarint(buf[:], uint64(val))
	if _, err := w.Write(buf[:n]); err != nil {
		return err
	}
	/*
		for val >= 128 {
			if err := w.WriteByte(byte(val & 127)); err != nil {
				return err
			}
			val = val >> 7
		}
		if err := w.WriteByte(byte(val & 127)); err != nil {
			return err
		}
	*/
	return nil
}
