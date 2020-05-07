package git

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"time"
	"unsafe"

	"sync"
	"sync/atomic"

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

	// Act as if reading from a non-seekable stream, not a file.
	Stdin bool
}

type PackfileIndex interface {
	GetObject(i io.ReaderAt, s Sha1) (GitObject, error)
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

	// the objects stream goes here in the file

	// The trailer from a V1 checksum
	Packfile, IdxFile Sha1
}

// Gets a list of objects in a pack file according to the index.
func v2PackObjectListFromIndex(idx io.Reader) []Sha1 {
	var pack PackfileIndexV2
	binary.Read(idx, binary.BigEndian, &pack.magic)
	binary.Read(idx, binary.BigEndian, &pack.Version)
	binary.Read(idx, binary.BigEndian, &pack.Fanout)
	pack.Sha1Table = make([]Sha1, pack.Fanout[255])
	// Load the tables. The first three are based on the number of
	// objects in the packfile (stored in Fanout[255]), the last
	// table is dynamicly sized.

	for i := 0; i < len(pack.Sha1Table); i++ {
		if err := binary.Read(idx, binary.BigEndian, &pack.Sha1Table[i]); err != nil {
			panic(err)
		}
	}
	return pack.Sha1Table
}

// reads a v2 pack file from r and tells if it has object inside it.
func v2PackIndexHasSha1(c *Client, pfile File, r io.Reader, obj Sha1) bool {
	var pack PackfileIndexV2
	binary.Read(r, binary.BigEndian, &pack.magic)
	binary.Read(r, binary.BigEndian, &pack.Version)
	binary.Read(r, binary.BigEndian, &pack.Fanout)
	pack.Sha1Table = make([]Sha1, pack.Fanout[255])
	pack.CRC32 = make([]uint32, pack.Fanout[255])
	pack.FourByteOffsets = make([]uint32, pack.Fanout[255])
	// Load the tables. The first three are based on the number of
	// objects in the packfile (stored in Fanout[255]), the last
	// table is dynamicly sized.

	for i := 0; i < len(pack.Sha1Table); i++ {
		if err := binary.Read(r, binary.BigEndian, &pack.Sha1Table[i]); err != nil {
			panic(err)
		}
	}
	for i := 0; i < len(pack.CRC32); i++ {
		if err := binary.Read(r, binary.BigEndian, &pack.CRC32[i]); err != nil {
			panic(err)
		}
	}
	for i := 0; i < len(pack.FourByteOffsets); i++ {
		if err := binary.Read(r, binary.BigEndian, &pack.FourByteOffsets[i]); err != nil {
			panic(err)
		}
		var offset int64
		if pack.FourByteOffsets[i]&(1<<31) != 0 {
			// clear out the MSB to get the offset
			eightbyteOffset := pack.FourByteOffsets[i] ^ (1 << 31)
			if eightbyteOffset&(1<<31) != 0 {
				var val uint64
				binary.Read(r, binary.BigEndian, &val)
				pack.EightByteOffsets = append(pack.EightByteOffsets, val)
				offset = int64(val)
			}
		} else {
			offset = int64(pack.FourByteOffsets[i])
		}
		c.objectCache[pack.Sha1Table[i]] = objectLocation{false, pfile, &pack, offset}
	}
	return pack.HasObject(obj)
}

func (idx PackfileIndexV2) WriteIndex(w io.Writer) error {
	return idx.writeIndex(w, true)
}

// Using the index, retrieve an object from the packfile represented by r at offset
// offset.
func (idx PackfileIndexV2) getObjectAtOffset(r io.ReaderAt, offset int64, metaOnly bool) (GitObject, error) {
	var p PackfileHeader

	// 4k should be enough for the header.
	metareader := io.NewSectionReader(r, offset, 4096)
	t, sz, ref, refoffset, rawheader := p.ReadHeaderSize(metareader)
	var rawdata []byte
	// sz is the uncompressed size, so the total size should usually be
	// less than sz for the compressed data. It might theoretically be a
	// little more, but we're generous here since this doesn't allocate
	// anything but just determines how much data the SectionReader will
	// read before returning an EOF.
	//
	// There is still overhead if the underlying ReaderAt reads more data
	// than it needs to and then discards it, so we assume that it won't
	// compress to more than double its original size, and then add a floor
	// of at least 1 disk sector since small objects are more likely to hit
	// degenerate cases for compression, but also less affected by the
	// multplication fudge factor, while a floor of 1 disk sector shouldn't
	// have much effect on disk IO (hopefully.)
	if sz != 0 {
		worstdsize := sz * 2
		if worstdsize < 512 {
			worstdsize = 512
		}
		datareader := io.NewSectionReader(r, offset+int64(len(rawheader)), int64(worstdsize))
		if !metaOnly || t == OBJ_OFS_DELTA || t == OBJ_REF_DELTA {
			rawdata = p.readEntryDataStream1(bufio.NewReader(datareader))
		}
	} else {
		// If it's size 0, sz*3 would immediately return io.EOF and cause
		// panic, so we just directly make the rawdata slice.
		rawdata = make([]byte, 0)
	}

	// The way we calculate the hash changes based on if it's a delta
	// or not.
	switch t {
	case OBJ_COMMIT:
		return GitCommitObject{int(sz), rawdata}, nil
	case OBJ_TREE:
		return GitTreeObject{int(sz), rawdata}, nil
	case OBJ_BLOB:
		return GitBlobObject{int(sz), rawdata}, nil
	case OBJ_OFS_DELTA:
		// Things aren't very consistent with if types are strings, types,
		// or interfaces, making this far more difficult than it needs to be.
		base, err := idx.getObjectAtOffset(r, offset-int64(refoffset), false)
		if err != nil {
			return nil, err
		}

		// calculateDelta needs a fully resolved delta, so we need to create
		// one based on the GitObject returned.
		res := resolvedDelta{Value: base.GetContent()}
		switch ty := base.GetType(); ty {
		case "commit":
			res.Type = OBJ_COMMIT
		case "tree":
			res.Type = OBJ_TREE
		case "blob":
			res.Type = OBJ_BLOB
		default:
			return nil, InvalidObject
		}

		baseType, val, err := calculateDelta(res, rawdata)
		if err != nil {
			return nil, err
		}
		// Convert back into a GitObject interface.
		switch baseType {
		case OBJ_COMMIT:
			return GitCommitObject{len(val), val}, nil
		case OBJ_TREE:
			return GitTreeObject{len(val), val}, nil
		case OBJ_BLOB:
			return GitBlobObject{len(val), val}, nil
		default:
			return nil, InvalidObject
		}
	case OBJ_REF_DELTA:
		base, err := idx.GetObject(r, ref)
		if err != nil {
			return nil, err
		}

		// calculateDelta needs a fully resolved delta, so we need to create
		// one based on the GitObject returned.
		res := resolvedDelta{Value: base.GetContent()}
		switch ty := base.GetType(); ty {
		case "commit":
			res.Type = OBJ_COMMIT
		case "tree":
			res.Type = OBJ_TREE
		case "blob":
			res.Type = OBJ_BLOB
		default:
			return nil, InvalidObject
		}

		baseType, val, err := calculateDelta(res, rawdata)
		if err != nil {
			return nil, err
		}
		// Convert back into a GitObject interface.
		switch baseType {
		case OBJ_COMMIT:
			return GitCommitObject{len(val), val}, nil
		case OBJ_TREE:
			return GitTreeObject{len(val), val}, nil
		case OBJ_BLOB:
			return GitBlobObject{len(val), val}, nil
		default:
			return nil, InvalidObject
		}
	default:
		return nil, fmt.Errorf("Unhandled object type.")
	}
}

// Find the object in the table.
func (idx PackfileIndexV2) GetObjectMetadata(r io.ReaderAt, s Sha1) (GitObject, error) {
	foundIdx := -1
	startIdx := idx.Fanout[s[0]]

	// Packfiles are designed so that we could do a binary search here, but
	// we don't need that optimization yet, so just do a linear search through
	// the objects with the same first byte.
	for i := startIdx - 1; idx.Sha1Table[i][0] == s[0]; i-- {
		if s == idx.Sha1Table[i] {
			foundIdx = int(i)
			break
		}
	}
	if foundIdx == -1 {
		return nil, fmt.Errorf("Object not found: %v", s)
	}

	var offset int64
	if idx.FourByteOffsets[foundIdx]&(1<<31) != 0 {
		// clear out the MSB to get the offset
		eightbyteOffset := idx.FourByteOffsets[foundIdx] ^ (1 << 31)
		offset = int64(idx.EightByteOffsets[eightbyteOffset])
	} else {
		offset = int64(idx.FourByteOffsets[foundIdx])
	}

	// Now that we've figured out where the object lives, use the packfile
	// to get the value from the packfile.
	return idx.getObjectAtOffset(r, offset, true)
}

func (idx PackfileIndexV2) GetObject(r io.ReaderAt, s Sha1) (GitObject, error) {
	foundIdx := -1
	startIdx := idx.Fanout[s[0]]
	if startIdx <= 0 {
		// The fanout table holds the number of entries less than x, so we
		// subtract 1 to make sure we don't miss the hash we're looking for,
		// but we need a special case for s[0] == 0 to prevent underflow
		startIdx = 1
	}

	// Packfiles are designed so that we could do a binary search here, but
	// we don't need that optimization yet, so just do a linear search through
	// the objects with the same first byte.
	for i := startIdx - 1; idx.Sha1Table[i][0] == s[0]; i-- {
		if s == idx.Sha1Table[i] {
			foundIdx = int(i)
			break
		}
	}
	if foundIdx == -1 {
		return nil, fmt.Errorf("Object not found: %v", s)
	}

	var offset int64
	if idx.FourByteOffsets[foundIdx]&(1<<31) != 0 {
		// clear out the MSB to get the offset
		eightbyteOffset := idx.FourByteOffsets[foundIdx] ^ (1 << 31)
		offset = int64(idx.EightByteOffsets[eightbyteOffset])
	} else {
		offset = int64(idx.FourByteOffsets[foundIdx])
	}

	// Now that we've figured out where the object lives, use the packfile
	// to get the value from the packfile.
	return idx.getObjectAtOffset(r, offset, false)
}

func getPackFileObject(idx io.Reader, packfile io.ReaderAt, s Sha1, metaOnly bool) (GitObject, error) {
	var pack PackfileIndexV2
	if err := binary.Read(idx, binary.BigEndian, &pack.magic); err != nil {
		return nil, err
	}
	if err := binary.Read(idx, binary.BigEndian, &pack.Version); err != nil {
		return nil, err
	}
	if err := binary.Read(idx, binary.BigEndian, &pack.Fanout); err != nil {
		return nil, err
	}
	pack.Sha1Table = make([]Sha1, pack.Fanout[255])
	pack.CRC32 = make([]uint32, pack.Fanout[255])
	pack.FourByteOffsets = make([]uint32, pack.Fanout[255])
	// Load the tables. The first three are based on the number of
	// objects in the packfile (stored in Fanout[255]), the last
	// table is dynamicly sized.

	for i := 0; i < len(pack.Sha1Table); i++ {
		if err := binary.Read(idx, binary.BigEndian, &pack.Sha1Table[i]); err != nil {
			return nil, err
		}
	}
	for i := 0; i < len(pack.CRC32); i++ {
		if err := binary.Read(idx, binary.BigEndian, &pack.CRC32[i]); err != nil {
			return nil, err
		}
	}
	for i := 0; i < len(pack.FourByteOffsets); i++ {
		if err := binary.Read(idx, binary.BigEndian, &pack.FourByteOffsets[i]); err != nil {
			return nil, err
		}
	}

	// The number of eight byte offsets is dynamic, based on how many
	// four byte offsets have the MSB set.
	for _, offset := range pack.FourByteOffsets {
		if offset&(1<<31) != 0 {
			var val uint64
			binary.Read(idx, binary.BigEndian, &val)
			pack.EightByteOffsets = append(pack.EightByteOffsets, val)
		}
	}
	if metaOnly {
		return pack.GetObjectMetadata(packfile, s)
	}
	return pack.GetObject(packfile, s)
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
	if startIdx <= 0 {
		// The fanout table holds the number of entries less than x, so we
		// subtract 1 to make sure we don't miss the hash we're looking for,
		// but we need a special case for s[0] == 0 to prevent underflow
		startIdx = 1
	}

	// Packfiles are designed so that we could do a binary search here, but
	// we don't need that optimization yet, so just do a linear search through
	// the objects with the same first byte.
	for i := int(startIdx - 1); i >= 0 && idx.Sha1Table[i][0] == s[0]; i-- {
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

func IndexPack(c *Client, opts IndexPackOptions, r io.Reader) (idx PackfileIndex, rerr error) {
	isfile := false
	if f, ok := r.(*os.File); ok && !opts.Stdin {
		// os.Stdin isn *os.File, but we want to consider it a stream.
		isfile = (f != os.Stdin)
	}

	// If --verbose is set, keep track of the time to output
	// a x kb/s in the output.
	var startTime time.Time
	if opts.Verbose {
		startTime = time.Now()
	}

	indexfile, initcb, icb := indexClosure(c, opts)

	cb := func(r io.ReaderAt, i, n int, loc int64, compsz int64, t PackEntryType, sz PackEntrySize, ref Sha1, offset ObjectOffset, rawdata []byte) error {
		if !isfile && opts.Verbose {
			now := time.Now()
			elapsed := now.Unix() - startTime.Unix()
			if elapsed == 0 {
				progressF("Receiving objects: %2.f%% (%d/%d)", (float32(i+1) / float32(n) * 100), i+1, n)
			} else {
				bps := loc / elapsed
				progressF("Receiving objects: %2.f%% (%d/%d), %v | %v/s", (float32(i+1) / float32(n) * 100), i+1, n, formatBytes(loc), formatBytes(bps))

			}
		}
		return icb(r, i, n, loc, compsz, t, sz, ref, offset, rawdata)
	}

	trailerCB := func(trailer Sha1) error {
		indexfile.Packfile = trailer

		sort.Sort(indexfile)
		// The sorting may have changed things, so as a final pass, hash
		// everything in the index to get the trailer (instead of doing it
		// while we were calculating it.)
		if err := indexfile.calculateTrailer(); err != nil {
			return err
		}
		return nil
	}

	pack, err := iteratePack(c, r, initcb, cb, trailerCB)
	if err != nil {
		return nil, err
	}
	defer pack.Close()

	// Write the index to disk and return
	var basename, idxname string
	if f, ok := r.(*os.File); ok && isfile && !opts.Stdin {
		basename = pack.Name()
		basename = strings.TrimSuffix(f.Name(), ".pack")
		idxname = basename + ".idx"
	} else {
		packhash, _ := indexfile.GetTrailer()
		basename := fmt.Sprintf("%s/pack-%s", c.GitDir.File("objects/pack").String(), packhash)
		idxname = basename + ".idx"

		if err := os.Rename(pack.Name(), basename+".pack"); err != nil {
			return indexfile, err
		}
	}

	if opts.Output == nil {
		o, err := os.Create(idxname)
		if err != nil {
			return indexfile, err
		}
		defer o.Close()
		opts.Output = o
	}

	if err := indexfile.WriteIndex(opts.Output); err != nil {
		return indexfile, err
	}
	return indexfile, err
}

func indexClosure(c *Client, opts IndexPackOptions) (*PackfileIndexV2, func(int), packIterator) {
	var indexfile PackfileIndexV2

	indexfile.magic = [4]byte{0377, 't', 'O', 'c'}
	indexfile.Version = 2

	var mu sync.Mutex

	priorObjects := make(map[Sha1]ObjectOffset)
	icb := func(n int) {
		indexfile.Sha1Table = make([]Sha1, n)
		indexfile.CRC32 = make([]uint32, n)
		indexfile.FourByteOffsets = make([]uint32, n)

	}

	cb := func(r io.ReaderAt, i, n int, location int64, compsz int64, t PackEntryType, sz PackEntrySize, ref Sha1, offset ObjectOffset, rawdata []byte) error {
		if opts.Verbose {
			progressF("Indexing objects: %2.f%% (%d/%d)", (float32(i+1) / float32(n) * 100), i+1, n)
		}

		checksum := crc32.NewIEEE()
		// The CRC32 checksum of the compressed data and the offset in
		// the file don't change regardless of type.
		rawcompressed := io.NewSectionReader(r, location, compsz)
		if _, err := io.Copy(checksum, rawcompressed); err != nil {
			return err
		}
		atomic.StoreUint32(&indexfile.CRC32[i], checksum.Sum32())

		if location < (1 << 31) {
			atomic.StoreUint32(&indexfile.FourByteOffsets[i], uint32(location))
		} else {
			atomic.StoreUint32(&indexfile.FourByteOffsets[i], uint32(len(indexfile.EightByteOffsets))|(1<<31))
			mu.Lock()
			indexfile.EightByteOffsets = append(indexfile.EightByteOffsets, uint64(location))
			mu.Unlock()
		}

		// The way we calculate the hash changes based on if it's a delta
		// or not.
		var sha1 Sha1
		switch t {
		case OBJ_COMMIT, OBJ_TREE, OBJ_BLOB, OBJ_TAG:
			sha1, _, err := HashSlice(t.String(), rawdata)
			if err != nil && opts.Strict {
				return err
			}
			for j := int(sha1[0]); j < 256; j++ {
				atomic.AddUint32(&indexfile.Fanout[j], 1)
			}

			// SHA1 is 160 bits.. since we know no one else is writing here,
			// we pretend it's 2 64 bit ints and a 32 bit int so that we can
			// use atomic writes instead of a lock.
			atomic.StoreUint64((*uint64)(unsafe.Pointer(&indexfile.Sha1Table[i][0])), *(*uint64)(unsafe.Pointer(&sha1[0])))
			atomic.StoreUint64((*uint64)(unsafe.Pointer(&indexfile.Sha1Table[i][8])), *(*uint64)(unsafe.Pointer(&sha1[8])))
			atomic.StoreUint32((*uint32)(unsafe.Pointer(&indexfile.Sha1Table[i][16])), *(*uint32)(unsafe.Pointer(&sha1[16])))

			// Maintain the list of references for further chains
			// to use.
			mu.Lock()
			priorObjects[sha1] = ObjectOffset(location)
			mu.Unlock()
		case OBJ_REF_DELTA:
			log.Printf("Resolving REF_DELTA from %v\n", ref)
			mu.Lock()
			o, ok := priorObjects[ref]
			mu.Unlock()
			if !ok {
				panic("Could not find basis for REF_DELTA")
			}
			// The refs in the index file need to be sorted in
			// order for GetObject to look up the other SHA1s
			// when resolving deltas. Chains don't have access
			// to the priorObjects map that we have here.
			mu.Lock()
			sort.Sort(&indexfile)
			mu.Unlock()

			base, err := indexfile.getObjectAtOffset(r, int64(o), false)
			if err != nil {
				return err
			}
			res := resolvedDelta{Value: base.GetContent()}
			_, val, err := calculateDelta(res, rawdata)
			if err != nil {
				return err
			}
			switch base.GetType() {
			case "commit":
				t = OBJ_COMMIT
			case "tree":
				t = OBJ_TREE
			case "blob":
				t = OBJ_BLOB
			default:
				panic("Unhandled delta base type" + base.GetType())
			}
			sha1, _, err = HashSlice(base.GetType(), val)
			if err != nil && opts.Strict {
				return err
			}

			mu.Lock()
			priorObjects[sha1] = ObjectOffset(location)
			mu.Unlock()

			for j := int(sha1[0]); j < 256; j++ {
				atomic.AddUint32(&indexfile.Fanout[j], 1)
			}

			// SHA1 is 160 bits.. since we know no one else is writing here,
			// we pretend it's 2 64 bit ints and a 32 bit int
			atomic.StoreUint64((*uint64)(unsafe.Pointer(&indexfile.Sha1Table[i][0])), *(*uint64)(unsafe.Pointer(&sha1[0])))
			atomic.StoreUint64((*uint64)(unsafe.Pointer(&indexfile.Sha1Table[i][8])), *(*uint64)(unsafe.Pointer(&sha1[8])))
			atomic.StoreUint32((*uint32)(unsafe.Pointer(&indexfile.Sha1Table[i][16])), *(*uint32)(unsafe.Pointer(&sha1[16])))
		case OBJ_OFS_DELTA:
			log.Printf("Resolving OFS_DELTA from %v\n", location-int64(offset))
			base, err := indexfile.getObjectAtOffset(r, location-int64(offset), false)
			if err != nil {
				return err
			}

			res := resolvedDelta{Value: base.GetContent()}
			_, val, err := calculateDelta(res, rawdata)
			if err != nil {
				return err
			}
			switch base.GetType() {
			case "commit":
				t = OBJ_COMMIT
			case "tree":
				t = OBJ_TREE
			case "blob":
				t = OBJ_BLOB
			default:
				panic("Unhandled delta base type" + base.GetType())
			}
			sha1, _, err = HashSlice(base.GetType(), val)
			if err != nil && opts.Strict {
				return err
			}

			mu.Lock()
			priorObjects[sha1] = ObjectOffset(location)
			mu.Unlock()

			for j := int(sha1[0]); j < 256; j++ {
				atomic.AddUint32(&indexfile.Fanout[j], 1)
			}

			// SHA1 is 160 bits.. since we know no one else is writing here,
			// we pretend it's 2 64 bit ints and a 32 bit int
			atomic.StoreUint64((*uint64)(unsafe.Pointer(&indexfile.Sha1Table[i][0])), *(*uint64)(unsafe.Pointer(&sha1[0])))
			atomic.StoreUint64((*uint64)(unsafe.Pointer(&indexfile.Sha1Table[i][8])), *(*uint64)(unsafe.Pointer(&sha1[8])))
			atomic.StoreUint32((*uint32)(unsafe.Pointer(&indexfile.Sha1Table[i][16])), *(*uint32)(unsafe.Pointer(&sha1[16])))
		default:
			panic("Unhandled type in IndexPack: " + t.String())
		}
		return nil
	}
	return &indexfile, icb, cb
}

// Indexes the pack, and stores a copy in Client's .git/objects/pack directory as it's
// doing so. This is the equivalent of "git index-pack --stdin", but works with any
// reader.
func IndexAndCopyPack(c *Client, opts IndexPackOptions, r io.Reader) (PackfileIndex, error) {
	return IndexPack(c, opts, r)
}

func formatBytes(n int64) string {
	if n <= 1024 {
		return fmt.Sprintf("%v B", n)
	} else if n <= 1024*1024 {
		return fmt.Sprintf("%.2f KiB", float64(n)/float64(1024))
	} else if n <= 1024*1024*1024 {
		return fmt.Sprintf("%.2f MiB", float64(n)/float64(1024*1024))
	}
	return fmt.Sprintf("%.2f GiB", float64(n)/float64(1024*1024*1024))
}
