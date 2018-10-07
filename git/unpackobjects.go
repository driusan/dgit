package git

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"sync"
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
func UnpackObjects(c *Client, opts UnpackObjectsOptions, r io.ReadSeeker) ([]Sha1, error) {
	var p PackfileHeader
	binary.Read(r, binary.BigEndian, &p)
	if p.Signature != [4]byte{'P', 'A', 'C', 'K'} {
		return nil, fmt.Errorf("Invalid packfile.")
	}
	if p.Version != 2 {
		return nil, fmt.Errorf("Unsupported packfile version: %d", p.Version)
	}

	var mu sync.Mutex

	// Store all the resolved OFS_DELTA values for resolving chains.
	ofsChains := make(map[ObjectOffset]resolvedDelta)

	// Store all the objects resolved references for REF_DELTA
	resolvedReferences := make(map[Sha1]resolvedDelta)

	// TODO: Replace this with the keys from the resolvedReferences map
	// instead of duplicating it.
	var objects []Sha1
	for i := uint32(0); i < p.Size; i += 1 {
		if !opts.Quiet {
			progressF("Unpacking objects: %2.f%% (%d/%d)", (float32(i+1) / float32(p.Size) * 100), i+1, p.Size)
		}
		start, err := r.Seek(0, io.SeekCurrent)
		if err != nil {
			if opts.Recover {
				log.Println(err)
				continue
			}
			return objects, err
		}
		t, s, ref, offset, _ := p.ReadHeaderSize(r)
		rawdata := p.readEntryDataStream1(r)
		switch t {
		case OBJ_COMMIT, OBJ_TREE, OBJ_BLOB:
			sha1, err := writeResolvedObject(c, t, rawdata)
			if err != nil {
				if opts.Recover {
					log.Println(err)
					continue
				}
				return objects, err
			}
			mu.Lock()
			objects = append(objects, sha1)
			resolvedReferences[sha1] = resolvedDelta{rawdata, t}
			ofsChains[ObjectOffset(start)] = resolvedDelta{rawdata, t}
			mu.Unlock()
		case OBJ_OFS_DELTA:
			t, deltadata, err := calculateOfsDelta(ObjectOffset(start)-offset, rawdata, ofsChains)

			if err != nil {
				if opts.Recover {
					log.Println(err)
					continue
				}
				return objects, err
			}
			mu.Lock()
			ofsChains[ObjectOffset(start)] = resolvedDelta{deltadata, t}
			mu.Unlock()
			switch t {
			case OBJ_COMMIT, OBJ_TREE, OBJ_BLOB:
				sha1, err := writeResolvedObject(c, t, deltadata)
				if err != nil {
					if opts.Recover {
						log.Println(err)
						continue
					}
					return objects, err
				}
				objects = append(objects, sha1)
			default:
				panic(fmt.Sprintf("TODO: Unhandled type %v", t))

			}
		case OBJ_REF_DELTA:
			t, deltadata, err := calculateRefDelta(ref, rawdata, resolvedReferences)
			if err != nil {
				if opts.Recover {
					log.Println(err)
					continue
				}
				return objects, err

			}

			switch t {
			case OBJ_COMMIT, OBJ_TREE, OBJ_BLOB:
				sha1, err := writeResolvedObject(c, t, deltadata)
				if err != nil {
					if opts.Recover {
						log.Println(err)
						continue
					}
					return objects, err
				}
				mu.Lock()
				objects = append(objects, sha1)
				resolvedReferences[sha1] = resolvedDelta{deltadata, t}
				mu.Unlock()
			default:
				panic(fmt.Sprintf("TODO: Unhandled type %v", t))
			}
		default:
			panic(fmt.Sprintf("TODO: Unhandled type %v", t))
		}

		if len(rawdata) != int(s) {
			panic(fmt.Sprintf("Incorrect size of entry %d: %d not %d", i, len(rawdata), s))
		}
	}
	return objects, nil
}
