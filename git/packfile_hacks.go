package git

import (
	"crypto/sha1"
	"encoding/binary"
	"io"
)

// Writes a packfile to w of the objects objects from the client's
// GitDir.
func SendPackfile(c *Client, w io.Writer, objects []Sha1) error {
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
		s := VariableLengthInt(obj.UncompressedSize(c))

		err := s.WriteVariable(w, obj.PackEntryType(c))
		if err != nil {
			return err
		}

		err = obj.CompressedWriter(c, w)
		if err != nil {
			return err
		}
	}
	trailer := sha.Sum(nil)
	w.Write(trailer)
	return nil
}
