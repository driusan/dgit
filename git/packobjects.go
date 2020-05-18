package git

import (
	"fmt"
	"io"

	"crypto/sha1"
	"encoding/binary"
)

type PackObjectsOptions struct {
	// The number of entries to use for the sliding window for delta
	// calculations
	Window int

	// Use offset deltas instead of refdeltas when calculating delta
	DeltaBaseOffset bool
}

// Writes a packfile to w of the objects objects from the client's
// GitDir.
func PackObjects(c *Client, opts PackObjectsOptions, w io.Writer, objects []Sha1) (trailer Sha1, err error) {
	if opts.Window != 0 {
		return Sha1{}, fmt.Errorf("Deltas not supported")
	}
	sha := sha1.New()
	w = io.MultiWriter(w, sha)
	n, err := w.Write([]byte{'P', 'A', 'C', 'K'})
	if n != 4 {
		panic("Could not write signature")
	}
	if err != nil {
		return Sha1{}, err
	}

	// Version
	binary.Write(w, binary.BigEndian, uint32(2))
	// Size
	binary.Write(w, binary.BigEndian, uint32(len(objects)))
	for _, obj := range objects {
		s := VariableLengthInt(obj.UncompressedSize(c))

		err := s.WriteVariable(w, obj.PackEntryType(c))
		if err != nil {
			return Sha1{}, err
		}

		err = obj.CompressedWriter(c, w)
		if err != nil {
			return Sha1{}, err
		}
	}
	trail := sha.Sum(nil)
	w.Write(trail)
	return Sha1FromSlice(trail)
}
