package git

import (
	"fmt"
	"io"
	"sort"
	"sync"

	"crypto/sha1"
	"encoding/binary"
	"hash/crc32"
)

type IndexPackOptions struct {
	// Display progress information while indexing pack.
	Verbose bool

	// Output index to this writer. If nil, will be based on
	// the filename.
	Output io.Writer

	// Fix a "thin" pack produced by git pack-objects --thin
	// (not implemented)
	FixThin bool

	// A message to store in a .keep file. The string "none"
	// will be interpreted as an empty file, the empty string
	// will be interpreted as do not produce a .keep file.
	Keep string

	// Not implemented
	IndexVersion int

	// Die if the pack contains broken links. (Not implemented)
	Strict bool

	// A number of threads to use for resolving deltas.  The 0-value
	// will use GOMAXPROCS.
	Threads uint
}

type PackfileIndex interface {
	HasObject(s Sha1) bool
	WriteIndex(w io.Writer) error
	GetTrailer() (Packfile Sha1, Index Sha1)
}

type PackIndexFanout [256]uint32
type PackfileIndexV2 struct {
	magic   [4]byte // Must be \377tOc
	Version uint32  // Must be 2

	Fanout PackIndexFanout

	Sha1Table []Sha1
	CRC32     []uint32

	// If the MSB is set, it's an index into the next
	// table, otherwise it's an index into the packfile.
	FourByteOffsets  []uint32
	EightByteOffsets []uint64

	// The trailer from a V1 checksum
	Packfile, IdxFile Sha1
}

// reads a v2 pack file from r and tells if it has object inside it.
// This avoids reading the entire pack file, since it only needs to
// read up to the Sha1 table.
func v2PackIndexHasSha1(r io.Reader, obj Sha1) bool {
	var pack PackfileIndexV2
	binary.Read(r, binary.BigEndian, &pack.magic)
	binary.Read(r, binary.BigEndian, &pack.Version)
	binary.Read(r, binary.BigEndian, &pack.Fanout)
	pack.Sha1Table = make([]Sha1, pack.Fanout[255])
	for i := 0; i < len(pack.Sha1Table); i++ {
		binary.Read(r, binary.BigEndian, &pack.Sha1Table[i])
	}

	return pack.HasObject(obj)
}
func (idx PackfileIndexV2) WriteIndex(w io.Writer) error {
	return idx.writeIndex(w, true)
}

func (idx PackfileIndexV2) GetTrailer() (Sha1, Sha1) {
	return idx.Packfile, idx.IdxFile
}

func (idx PackfileIndexV2) writeIndex(w io.Writer, withTrailer bool) error {
	if err := binary.Write(w, binary.BigEndian, idx.magic); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, idx.Version); err != nil {
		return err
	}
	for _, fanout := range idx.Fanout {
		if err := binary.Write(w, binary.BigEndian, fanout); err != nil {
			return err
		}
	}
	for _, sha := range idx.Sha1Table {
		if err := binary.Write(w, binary.BigEndian, sha); err != nil {
			return err
		}
	}
	for _, crc32 := range idx.CRC32 {
		if err := binary.Write(w, binary.BigEndian, crc32); err != nil {
			return err
		}
	}
	for _, offset := range idx.FourByteOffsets {
		if err := binary.Write(w, binary.BigEndian, offset); err != nil {
			return err
		}
	}
	for _, offset := range idx.EightByteOffsets {
		if err := binary.Write(w, binary.BigEndian, offset); err != nil {
			return err
		}
	}
	if err := binary.Write(w, binary.BigEndian, idx.Packfile); err != nil {
		return err
	}
	if withTrailer {
		if err := binary.Write(w, binary.BigEndian, idx.IdxFile); err != nil {
			return err
		}
	}
	return nil
}
func (idx PackfileIndexV2) HasObject(s Sha1) bool {
	startIdx := idx.Fanout[s[0]]

	// Packfiles are designed so that we could do a binary search here, but
	// we don't need that optimization yet, so just do a linear search through
	// the objects with the same first byte.
	for i := startIdx - 1; idx.Sha1Table[i][0] == s[0]; i-- {
		if s == idx.Sha1Table[i] {
			return true
		}
	}
	return false
}

// Implements the Sorter interface on PackfileIndexV2, in order to sort the
// Sha1, CRC32, and
func (p *PackfileIndexV2) Len() int {
	return int(p.Fanout[255])
}

func (p *PackfileIndexV2) Swap(i, j int) {
	p.Sha1Table[i], p.Sha1Table[j] = p.Sha1Table[j], p.Sha1Table[i]
	p.CRC32[i], p.CRC32[j] = p.CRC32[j], p.CRC32[i]
	p.FourByteOffsets[i], p.FourByteOffsets[j] = p.FourByteOffsets[j], p.FourByteOffsets[i]
}

func (p *PackfileIndexV2) Less(i, j int) bool {
	for k := 0; k < 20; k++ {
		if p.Sha1Table[i][k] < p.Sha1Table[j][k] {
			return true
		} else if p.Sha1Table[i][k] > p.Sha1Table[j][k] {
			return false
		}
	}
	return false
}

// calculates and stores the trailer into the packfile.
func (p *PackfileIndexV2) calculateTrailer() error {
	trailer := sha1.New()
	if err := p.writeIndex(trailer, false); err != nil {
		return err
	}
	t, err := Sha1FromSlice(trailer.Sum(nil))
	if err != nil {
		return err
	}
	p.IdxFile = t
	return nil
}

func IndexPack(c *Client, opts IndexPackOptions, r io.ReadSeeker) (PackfileIndex, error) {
	var p PackfileHeader
	binary.Read(r, binary.BigEndian, &p)
	if p.Signature != [4]byte{'P', 'A', 'C', 'K'} {
		return nil, fmt.Errorf("Invalid packfile.")
	}
	if p.Version != 2 {
		return nil, fmt.Errorf("Unsupported packfile version: %d", p.Version)
	}

	//	deltaChains := make(map[ObjectOffset]resolvedDelta)
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(int(p.Size))

	var indexfile PackfileIndexV2
	indexfile.magic = [4]byte{0377, 't', 'O', 'c'}
	indexfile.Version = 2

	indexfile.Sha1Table = make([]Sha1, p.Size)
	indexfile.CRC32 = make([]uint32, p.Size)
	indexfile.FourByteOffsets = make([]uint32, p.Size)
	ofsChains := make(map[ObjectOffset]resolvedDelta)
	refChains := make(map[Sha1]resolvedDelta)
	for i := uint32(0); i < p.Size; i += 1 {
		if opts.Verbose {
			progressF("Indexing objects: %2.f%% (%d/%d)", (float32(i+1) / float32(p.Size) * 100), i+1, p.Size)
		}
		location, err := r.Seek(0, io.SeekCurrent)
		if err != nil {
			// It's not worth trying to recover from being unable to
			// find the current location, since it's required for
			// the offset table.
			return nil, err
		}
		//t, s, ref, offset := p.ReadHeaderSize(r)
		t, _, ref, offset, rawheader := p.ReadHeaderSize(r)
		rawdata, compressed := p.ReadEntryDataStream(r)
		checksum := crc32.ChecksumIEEE(append(rawheader, compressed...))

		// The CRC32 checksum of the compressed data and the offset in
		// the file don't change regardless of type.
		mu.Lock()
		indexfile.CRC32[i] = checksum

		if location < (1 << 31) {
			indexfile.FourByteOffsets[i] = uint32(location)
		} else {
			indexfile.FourByteOffsets[i] = uint32(len(indexfile.EightByteOffsets)) | (1 << 31)
			indexfile.EightByteOffsets = append(indexfile.EightByteOffsets, uint64(location))
		}
		mu.Unlock()

		// The way we calculate the hash changes based on if it's a delta
		// or not.
		switch t {
		case OBJ_COMMIT, OBJ_TREE, OBJ_BLOB:
			sha1, _, err := HashSlice(t.String(), rawdata)
			if err != nil && opts.Strict {
				return indexfile, err
			}
			mu.Lock()
			for j := int(sha1[0]); j < 256; j++ {
				indexfile.Fanout[j]++
			}
			indexfile.Sha1Table[i] = sha1
			mu.Unlock()

			// Maintain the list of references for further chains
			// to use.
			mu.Lock()
			refChains[sha1] = resolvedDelta{rawdata, t}
			ofsChains[ObjectOffset(location)] = resolvedDelta{rawdata, t}
			mu.Unlock()

			wg.Done()
		case OBJ_OFS_DELTA:
			t, deltadata, err := calculateOfsDelta(ObjectOffset(location)-offset, rawdata, ofsChains)
			if err != nil && opts.Strict {
				return indexfile, err
			}
			mu.Lock()
			ofsChains[ObjectOffset(location)] = resolvedDelta{deltadata, t}
			mu.Unlock()
			if err != nil && opts.Strict {
				return nil, err
			}
			sha1, _, err := HashSlice(t.String(), deltadata)
			if err != nil && opts.Strict {
				return nil, err
			}

			mu.Lock()
			for j := int(sha1[0]); j < 256; j++ {
				indexfile.Fanout[j]++
			}
			indexfile.Sha1Table[i] = sha1

			mu.Unlock()
			wg.Done()
		case OBJ_REF_DELTA:
			t, deltadata, err := calculateRefDelta(ref, rawdata, refChains)
			if err != nil && opts.Strict {
				return indexfile, err
			}
			if err != nil && opts.Strict {
				return nil, err
			}
			sha1, _, err := HashSlice(t.String(), deltadata)
			mu.Lock()
			refChains[sha1] = resolvedDelta{deltadata, t}
			mu.Unlock()

			if err != nil && opts.Strict {
				return nil, err
			}

			mu.Lock()
			for j := int(sha1[0]); j < 256; j++ {
				indexfile.Fanout[j]++
			}
			indexfile.Sha1Table[i] = sha1
			mu.Unlock()
			wg.Done()
		default:
			panic("Unhandled type in IndexPack")

		}
	}
	/* It's probably premature to require a Go 1.8 feature
	sort.Slice(index, func(i, j int) bool {
		return index[i] < index[j]
	})*/
	// Read the packfile trailer into the index trailer.
	binary.Read(r, binary.BigEndian, &indexfile.Packfile)
	sort.Sort(&indexfile)

	// The sorting may have changed things, so as a final pass, hash
	// everything to get the trailer (instead of doing it while we
	// were calculating everything.)
	err := indexfile.calculateTrailer()
	return indexfile, err
}
