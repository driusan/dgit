package git

import (
	"bytes"
	"fmt"
	"io"
	//"io/ioutil"
	"log"
	"os"
	"runtime/pprof"

	"compress/zlib"
	"crypto/sha1"
	"encoding/binary"

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
type previousObject struct {
	oid      Sha1
	location int
}

// Writes a packfile to w of the objects objects from the client's
// GitDir.
func PackObjects(c *Client, opts PackObjectsOptions, w io.Writer, objects []Sha1) (trailer Sha1, err error) {
	f, err := os.Create("profile.cpu")
	if err != nil {
		log.Fatal("could not create CPU profile: ", err)
	}
	defer f.Close() // error handling omitted for example
	if err := pprof.StartCPUProfile(f); err != nil {
		log.Fatal("could not start CPU profile: ", err)
	}
	defer pprof.StopCPUProfile()
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
	for i, obj := range objects {
		objcontent, err := c.GetObject(obj)
		if err != nil {
			return Sha1{}, err
		}
		objbytes := objcontent.GetContent()
		best := objbytes
		otyp := obj.PackEntryType(c)
		otyp1 := obj.PackEntryType(c)
		var offref *Sha1

		// We don't bother trying to calculate how close the object
		// is, we just blindly calculate a delta and calculate the
		// size.

		for tryidx := i - 1; tryidx >= 0 && tryidx > i-opts.Window; tryidx-- {
			trytyp := obj.PackEntryType(c)
			if trytyp != otyp1 {
				continue
			}
			base, err := c.GetObject(objects[tryidx])
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}
			basebytes := base.GetContent()
			var newdelta bytes.Buffer
			if err := delta.Calculate(&newdelta, basebytes, objbytes, len(best) / 2); err == nil {

				if d := newdelta.Bytes(); len(d) < len(best) {
					best = d
					otyp = OBJ_REF_DELTA
					offref = &objects[tryidx]
				}
			} else {
				log.Println(err)
			}
		}
		s := VariableLengthInt(len(best))

		if err := s.WriteVariable(w, otyp); err != nil {
			return Sha1{}, err
		}

		if offref != nil {
			derefoff := Sha1(*offref)
			if n, err := w.Write(derefoff[:]); err != nil {
				return Sha1{}, err
			} else if n != 20 {
				panic("could not write ref offset")
			}
		}
		zw := zlib.NewWriter(w)
		if err != nil {
			panic(err)
		}
		if _, err := zw.Write(best); err != nil {
			return Sha1{}, err
		}
		zw.Close()
	}
	trail := sha.Sum(nil)
	w.Write(trail)
	return Sha1FromSlice(trail)
}
