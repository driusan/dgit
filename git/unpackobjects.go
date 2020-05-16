package git

import (
	// "encoding/binary"
	"container/list"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	//"bytes"
	"log"
	// "sync"
)

type UnpackObjectsOptions struct {
	// Do not write any objects (not implemented)
	DryRun bool

	// Do not print any progress information to os.Stderr
	Quiet bool

	// Attempt to recover corrupt pack files (not implemented)
	Recover bool

	// Do not write objects with broken content or links (not implemented)
	Strict bool

	// Do not attempt to process packfiles larger than this size.
	// (the value "0" means unlimited.)
	MaxInputSize uint
}

// Unpack the objects from r's input stream into the client GitDir's
// objects directory and returns the list of objects that were unpacked.
func UnpackObjects(c *Client, opts UnpackObjectsOptions, r io.Reader) ([]Sha1, error) {
	var shas []Sha1
	deltas := list.New()

	// We don't actually use this index, it just let's us use getObjectAtLocationForIndexing
	// (which also doesn't use the index)
	var idx PackfileIndexV2
	// For REF_DELTA to resolve.
	priorObjects := make(map[Sha1]*packObject)
	// For OFS_DELTA to resolve.
	priorLocations := make(map[ObjectOffset]*packObject)
	cb := func(r io.ReaderAt, i, n int, location int64, t PackEntryType, sz PackEntrySize, refSha1 Sha1, offset ObjectOffset, rawdata []byte) error {
		if !opts.Quiet {
			progressF("Unpacking objects: %2.f%% (%d/%d)", i+1 == n, (float32(i+1)/float32(n))*100, i+1, n)
		}
		switch t {
		case OBJ_COMMIT, OBJ_TREE, OBJ_BLOB, OBJ_TAG:
			sha1, err := c.WriteObject(t.String(), rawdata)
			if err != nil {
				return err
			}
			shas = append(shas, sha1)

			objCache := &packObject{
				idx:      i,
				oid:      sha1,
				location: ObjectOffset(location),
			}
			if o, ok := priorObjects[sha1]; !ok {
				priorObjects[sha1] = objCache
			} else {
				// We have the lock and we know no one is reading
				// these until we're done the first round of
				// indexing anyways, so we don't bother to use
				// the atomic package.
				o.location = ObjectOffset(location)
				o.idx = i
			}
			priorLocations[objCache.location] = objCache
			return nil
		case OBJ_REF_DELTA:
			log.Printf("Noting REF_DELTA to resolve: %v\n", refSha1)
			o, ok := priorObjects[refSha1]
			if !ok {
				// It hasn't been seen yet, so just note
				// that there's a a delta against it for
				// later.
				// Since we haven't seen it yet, we don't
				// have a location.
				objCache := &packObject{
					oid:            refSha1,
					deltasAgainst:  1,
					deltasResolved: 0,
				}
				priorObjects[refSha1] = objCache
			} else {
				o.deltasAgainst += 1
			}
			self := &packObject{
				idx:            i,
				location:       ObjectOffset(location),
				deltasAgainst:  0,
				deltasResolved: 0,
				typ:            t,
			}
			priorLocations[ObjectOffset(location)] = self
			deltas.PushBack(self)
			return nil
		case OBJ_OFS_DELTA:
			log.Printf("Noting OFS_DELTA to resolve from %v\n", location-int64(offset))
			// Adjust the number of deltas against the parent
			// priorLocations should always be populated with
			// the prior objects (even if some fields aren't
			// populated), and offets are always looking back
			// into the packfile, so this shouldn't happen.
			if o, ok := priorLocations[ObjectOffset(location-int64(offset))]; !ok {
				panic("Can not determine delta base")
			} else {
				o.deltasAgainst += 1
			}

			// Add ourselves to the map for future deltas
			self := &packObject{
				idx:            i,
				location:       ObjectOffset(location),
				deltasAgainst:  0,
				deltasResolved: 0,
				baselocation:   ObjectOffset(location) - ObjectOffset(offset),
				typ:            t,
			}
			priorLocations[ObjectOffset(location)] = self
			deltas.PushBack(self)
			return nil
		default:
			return fmt.Errorf("Unhandled object type: %v", t)
		}
	}

	trailerCB := func(r io.ReaderAt, packn int, packtrailer Sha1) error {
		var e error
		for el := deltas.Front(); el != nil; el = el.Next() {
			delta := el.Value.(*packObject)

			t, r, _, err := idx.getObjectAtOffsetForIndexing(r, int64(delta.location), false, priorLocations, priorObjects)
			if err != nil {
				if opts.Recover {
					log.Println(err)
					e = err
					continue
				}
				return err
			}

			data, err := ioutil.ReadAll(r)
			if opts.Recover {
				log.Println(err)
				e = err
				continue
			}
			if sha, err := c.WriteObject(t.String(), data); err != nil {
				if opts.Recover {
					log.Println(err)
					e = err
					continue
				}
			} else {
				delta.oid = sha
				priorObjects[sha] = delta
				shas = append(shas, sha)
			}
		}
		return e
	}

	// No-ops while unpacking
	initcb := func(n int) {
	}
	crc32cb := func(i int, crc uint32) error {
		return nil
	}

	pack, err := iteratePack(c, r, initcb, cb, trailerCB, crc32cb)
	if err != nil {
		return shas, err
	}

	// If r was a file iteratePack reused it as the ReaderAt, so
	// don't delete it.
	if f, ok := r.(*os.File); !ok || f == os.Stdin {
		os.Remove(pack.Name())
		return shas, nil
	}
	return shas, nil
}
