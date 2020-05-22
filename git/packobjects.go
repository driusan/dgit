package git

import (
	"bytes"
	"io"
	"log"
	//"os"
	//"runtime/pprof"

	"compress/zlib"
	"crypto/sha1"
	"encoding/binary"
	"index/suffixarray"

	"github.com/driusan/dgit/git/delta"
)

type PackObjectsOptions struct {
	// The number of entries to use for the sliding window for delta
	// calculations
	Window int

	// Use offset deltas instead of refdeltas when calculating delta
	DeltaBaseOffset bool
}

// Used for keeping track of the previous window objects to encode
// their location with DeltaBaseOffset set
type packWindow struct {
	oid      Sha1
	location int
	typ      PackEntryType
	cache    []byte
	index    *suffixarray.Index
}

// Writes a packfile to w of the objects objects from the client's
// GitDir.
func PackObjects(c *Client, opts PackObjectsOptions, w io.Writer, objects []Sha1) (trailer Sha1, err error) {
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
	var window []packWindow = make([]packWindow, 0, opts.Window)

	pos := 12 // PACK + uint32 + uint32
	for i, obj := range objects {
		objcontent, err := c.GetObject(obj)
		if err != nil {
			return Sha1{}, err
		}
		objbytes := objcontent.GetContent()
		best := objbytes
		otyp := obj.PackEntryType(c)
		otypreal := obj.PackEntryType(c)
		written := 0
		var ref *packWindow

		// We don't bother trying to calculate how close the object
		// is, we just blindly calculate a delta and calculate the
		// size.
		for _, tryobj := range window {
			basebytes := tryobj.cache
			if tryobj.typ != otypreal {
				continue
			}

			var newdelta bytes.Buffer
			if err := delta.CalculateWithIndex(tryobj.index, &newdelta, basebytes, objbytes, len(best)/2); err == nil {

				if d := newdelta.Bytes(); len(d) < len(best) {
					best = d
					if opts.DeltaBaseOffset {
						otyp = OBJ_OFS_DELTA
					} else {
						otyp = OBJ_REF_DELTA
					}
					ref = &tryobj
				}
			} else {
				log.Println(err)
			}
		}

		s := VariableLengthInt(len(best))

		if n, err := s.WriteVariable(w, otyp); err != nil {
			return Sha1{}, err
		} else {
			written += n
		}

		if ref != nil {
			if opts.DeltaBaseOffset {
				offset := pos - ref.location
				println("Offset", pos-ref.location)
				var buf [128]byte
				n := binary.PutUvarint(buf[:], uint64(offset))
				n, err := w.Write(buf[:n])
				if err != nil {
					return Sha1{}, err
				}
				written += n

			} else {
				if _, err := w.Write(ref.oid[:]); err != nil {
					return Sha1{}, err
				}
				written += 20
			}
		}

		// We write into a buffer so that we can calcualte the compressed
		// size easily. the n from io.Writer is the uncompressed size.
		var cbuf bytes.Buffer
		zw := zlib.NewWriter(&cbuf)
		if err != nil {
			panic(err)
		}
		if _, err := zw.Write(best); err != nil {
			return Sha1{}, err
		}
		zw.Close()

		if _, err := w.Write(cbuf.Bytes()); err != nil {
			return Sha1{}, err
		}

		written += cbuf.Len()

		suffix := suffixarray.New(objbytes)
		if opts.Window > 0 {
			if i < opts.Window {
				window = append(window, packWindow{
					oid:      obj,
					location: pos,
					typ:      otypreal,
					cache:    objbytes,
					index:    suffix,
				})
			} else {
				window[i%opts.Window] = packWindow{
					oid:      obj,
					location: pos,
					typ:      otypreal,
					cache:    objbytes,
					index:    suffix,
				}
			}
		}

		pos += written
	}
	trail := sha.Sum(nil)
	w.Write(trail)
	return Sha1FromSlice(trail)
}
