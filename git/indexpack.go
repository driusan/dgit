package git

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"
	"unsafe"

	"compress/flate"

	"sync"
	"sync/atomic"

	"crypto/sha1"
	"encoding/binary"
	// "hash/crc32"
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

// Using the index, retrieve an object from the packfile represented by r
// at offset. The index must be valid for this function to work, it can
// not retrieve objects before the index is built (ie. during
// `git index-pack`).
func (idx PackfileIndexV2) getObjectAtOffset(r io.ReaderAt, offset int64, metaOnly bool) (rv GitObject, err error) {
	var p PackfileHeader

	// 4k should be enough for the header.
	metareader := io.NewSectionReader(r, offset, 4096)
	t, sz, ref, refoffset, rawheader := p.ReadHeaderSize(bufio.NewReader(metareader))
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
		o := GitCommitObject{int(sz), rawdata}
		return o, nil
	case OBJ_TREE:
		o := GitTreeObject{int(sz), rawdata}
		return o, nil
		return GitTreeObject{int(sz), rawdata}, nil
	case OBJ_BLOB:
		o := GitBlobObject{int(sz), rawdata}
		return o, nil
	case OBJ_TAG:
		o := GitTagObject{int(sz), rawdata}
		return o, nil
	case OBJ_OFS_DELTA:
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
		case "tag":
			res.Type = OBJ_TAG
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
		case OBJ_TAG:
			return GitTagObject{len(val), val}, nil
		default:
			return nil, InvalidObject
		}
	case OBJ_REF_DELTA:
		var base GitObject
		// This function is only after the index is built, so
		// it should have all referenced objects.
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
		case "tag":
			res.Type = OBJ_TAG
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
		case OBJ_TAG:
			return GitTagObject{len(val), val}, nil
		default:
			return nil, InvalidObject
		}
	default:
		return nil, fmt.Errorf("Unhandled object type.")
	}
}

type lrucache struct {
	head, tail *cachenode

	offsetmap map[ObjectOffset]*cachenode

	sz    int // total size in bytes of objects stored
	maxsz int
}

type cachenode struct {
	next, prev *cachenode

	offset ObjectOffset

	ResolvedType PackEntryType
	Header       []byte
	Data         []byte
}

func (c *lrucache) Get(offset ObjectOffset) (PackEntryType, []byte) {
	if o, ok := c.offsetmap[offset]; ok {
		c.moveToFront(o)
		return o.ResolvedType, o.Data
	}
	return 0, nil
}

func (c *lrucache) moveToFront(n *cachenode) {
	c.head.next = n
	n.next.prev = n.prev
	n.prev.next = n.next

	n.prev = c.head
	n.next = nil
	c.head = n
	if c.tail == nil {
		c.tail = n
	}
	return
}

func (c *lrucache) add(offset ObjectOffset, n *cachenode) {
	sz := len(n.Data)
	if sz > c.maxsz {
		// If it will take up more than half our cache, don't
		// cache it.
		return
	}
	if sz+c.sz > c.maxsz {
		// When we reach the max capacity we free half the
		// cache. The memory won't be freed until the GC is
		// run. Once we reach capacity, we'll being reaching
		// it often when adding new objects if we free just
		// enough, and constantly running the GC defeats the
		// purpose of having a cache.
		var mstati runtime.MemStats
		runtime.ReadMemStats(&mstati)
		for (sz + c.sz) > c.maxsz/2 {
			// println("sz", sz, " c.sz", c.sz, " maxcz", c.maxsz)
			// Remove tail
			oldtail := c.tail
			var newtail *cachenode
			if oldtail != nil {
				newtail = oldtail.next
			}
			if c.tail == nil {
				return
				panic("No tail to remove")
			} else if c.tail.Data == nil {
				panic("Tail doesn't have content")
			}

			c.sz -= len(oldtail.Data)
			if newtail != nil {
				newtail.prev = nil

				// Make sure GC picks up the node
				// by removing any potential references
				if oldtail.next != nil {
					oldtail.next.prev = nil
				}
				if oldtail.prev != nil {
					oldtail.prev.next = nil
				}
			}

			delete(c.offsetmap, oldtail.offset)
			c.tail = newtail
		}
		runtime.GC()
		var mstat runtime.MemStats
		runtime.ReadMemStats(&mstat)
		if mstat.Alloc > 1271664312 {
			// FIXME: Find memory leak
			ocache = newObjectCache()
			runtime.GC()
		}
		println("Original heap", mstati.Alloc, " new heap", mstat.Alloc)
	}
	c.offsetmap[offset] = n
	n.prev = c.head
	if n.prev != nil {
		n.prev.next = n
	}
	n.next = nil

	c.head = n
	if c.tail == nil {
		c.tail = n
	}
	c.sz += sz
}

func (c *lrucache) Cache(offset ObjectOffset, typ PackEntryType, header, data []byte) {
	n := c.offsetmap[offset]
	if n != nil {
		// Tried to re-cache the same thing, just move it to the front
		c.moveToFront(n)
		return
	}
	// Add to cache if applicable
	n = &cachenode{
		prev:         c.head,
		ResolvedType: typ,
		Header:       header,
		Data:         data,
	}
	c.add(offset, n)
	return
}

func newObjectCache() lrucache {
	c := lrucache{
		offsetmap: make(map[ObjectOffset]*cachenode),
		//maxsz:     256 * 1024 * 1024,
		maxsz: 64 * 1024 * 1024,
		// maxsz: 2 * 1024 * 1024, // testing
	}
	return c
}

var ocache lrucache = newObjectCache()

// Retrieve an object from the packfile represented by r at offset.
// This will use the specified caches to resolve the location of any
// deltas, not the index itself. They must be maintained by the caller.
func (idx PackfileIndexV2) getObjectAtOffsetForIndexing(r io.ReaderAt, offset int64, metaOnly bool, cache map[ObjectOffset]*packObject, refcache map[Sha1]*packObject) (t PackEntryType, data io.Reader, osz int64, err error) {
	if t, data := ocache.Get(ObjectOffset(offset)); data != nil {
		// println("Using cache")
		return t, bytes.NewReader(data), int64(len(data)), nil
	}

	var p PackfileHeader

	// 4k should be enough for the header.
	var datareader flate.Reader
	metareader := io.NewSectionReader(r, offset, 4096)
	//t, sz, ref, refoffset, rawheader := p.ReadHeaderSize(metareader)
	t, sz, ref, refoffset, rawheader := p.ReadHeaderSize(bufio.NewReader(metareader))
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
		if !metaOnly || t == OBJ_OFS_DELTA || t == OBJ_REF_DELTA {
			// readDataEntryStream needs a ByteReader, so we wrap
			// the reader in a bufio
			dr, err := p.dataStream(bufio.NewReader(io.NewSectionReader(r, offset+int64(len(rawheader)), int64(worstdsize))))
			if err != nil {
				return 0, nil, 0, err
			}
			datareader = bufio.NewReader(dr)
		}
	} else {
		// If it's size 0, sz*3 would immediately return io.EOF and cause
		// panic, so we just directly make the rawdata slice.
		datareader = bytes.NewBuffer(nil)
	}

	// The way we calculate the hash changes based on if it's a delta
	// or not.
	switch t {
	case OBJ_COMMIT, OBJ_TREE, OBJ_BLOB, OBJ_TAG:
		return t, datareader, int64(sz), nil
	case OBJ_REF_DELTA:
		parent := refcache[ref]
		parent.deltasResolved++
		t, r, _, err := idx.getObjectAtOffsetForIndexing(r, int64(parent.location), false, cache, refcache)
		if err != nil {
			return 0, nil, 0, err
		}
		if t == OBJ_OFS_DELTA || t == OBJ_REF_DELTA {
			panic("No chains")
		}
		base, err := ioutil.ReadAll(r)
		if err != nil {
			return 0, nil, 0, err
		}
		deltareader := newDelta(datareader, bytes.NewReader(base))
		return t, &deltareader, 0, err
	case OBJ_OFS_DELTA:
		parent := cache[ObjectOffset(offset-int64(refoffset))]
		parent.deltasResolved++
		t, r, _, err := idx.getObjectAtOffsetForIndexing(r, offset-int64(refoffset), false, cache, refcache)
		if err != nil {
			return 0, nil, 0, err
		}
		if t == OBJ_OFS_DELTA || t == OBJ_REF_DELTA {
			panic("No chains")
		}
		base, err := ioutil.ReadAll(r)
		if err != nil {
			return 0, nil, 0, err
		}
		deltareader := newDelta(datareader, bytes.NewReader(base))
		return t, &deltareader, 0, err
	default:
		return 0, nil, 0, fmt.Errorf("Unhandled object type %v: ", t)
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

func (idx PackfileIndexV2) getObject(r io.ReaderAt, s Sha1) (GitObject, error) {
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

func (idx PackfileIndexV2) GetObject(r io.ReaderAt, s Sha1) (GitObject, error) {
	return idx.GetObject(r, s)
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

	var deltas []*packObject
	indexfile, initcb, icb, crc32cb, priorObjects, priorLocations := indexClosure(c, opts, &deltas)

	cb := func(r io.ReaderAt, i, n int, loc int64, t PackEntryType, sz PackEntrySize, ref Sha1, offset ObjectOffset, header, data []byte) error {
		if !isfile && opts.Verbose {
			now := time.Now()
			elapsed := now.Unix() - startTime.Unix()
			if elapsed == 0 {
				progressF("Receiving objects: %2.f%% (%d/%d)", i+1 == n, (float32(i+1) / float32(n) * 100), i+1, n)
			} else {
				bps := loc / elapsed
				progressF("Receiving objects: %2.f%% (%d/%d), %v | %v/s", i+1 == n, (float32(i+1) / float32(n) * 100), i+1, n, formatBytes(loc), formatBytes(bps))

			}
		}
		return icb(r, i, n, loc, t, sz, ref, offset, header, data)
	}

	trailerCB := func(r io.ReaderAt, n int, trailer Sha1) error {
		for i, delta := range deltas {
			if opts.Verbose {
				progressF("Resolving deltas: %2.f%% (%d/%d)", i+1 == len(deltas), (float32(i+1) / float32(len(deltas)) * 100), i+1, len(deltas))
			}

			t, r, sz, err := indexfile.getObjectAtOffsetForIndexing(r, int64(delta.location), false, priorLocations, priorObjects)
			if err != nil {
				return err
			}

			sha1, err := HashReaderWithSize(t.String(), sz, r)
			if err != nil {
				return err
			}
			delta.oid = sha1
			priorObjects[sha1] = delta
			indexfile.Sha1Table[delta.idx] = sha1
			for j := int(sha1[0]); j < 256; j++ {
				atomic.AddUint32(&indexfile.Fanout[j], 1)
			}
		}

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

	pack, err := iteratePack(c, r, initcb, cb, trailerCB, crc32cb)
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

type packObject struct {
	idx                           int
	oid                           Sha1
	location                      ObjectOffset
	deltasAgainst, deltasResolved int
	baselocation                  ObjectOffset
	typ                           PackEntryType
}

func indexClosure(c *Client, opts IndexPackOptions, deltas *[]*packObject) (*PackfileIndexV2, func(int), packIterator, func(int, uint32) error, map[Sha1]*packObject, map[ObjectOffset]*packObject) {
	var indexfile PackfileIndexV2

	indexfile.magic = [4]byte{0377, 't', 'O', 'c'}
	indexfile.Version = 2

	var mu sync.Mutex

	// For REF_DELTA to resolve
	priorObjects := make(map[Sha1]*packObject)
	// For OFS_DELTA to resolve
	priorLocations := make(map[ObjectOffset]*packObject)

	icb := func(n int) {
		*deltas = make([]*packObject, 0, n)
		indexfile.Sha1Table = make([]Sha1, n)
		indexfile.CRC32 = make([]uint32, n)
		indexfile.FourByteOffsets = make([]uint32, n)
	}

	cb := func(r io.ReaderAt, i, n int, location int64, t PackEntryType, sz PackEntrySize, ref Sha1, offset ObjectOffset, rawheader, rawdata []byte) error {
		if opts.Verbose {
			progressF("Indexing objects: %2.f%% (%d/%d)", i+1 == n, (float32(i+1) / float32(n) * 100), i+1, n)
		}

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
			ocache.Cache(ObjectOffset(location), t, rawheader, rawdata)
			sha1, err := HashReaderWithSize(t.String(), int64(len(rawdata)), bytes.NewReader(rawdata))
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

			// Maintain the list of references for delta chains.
			// There's a possibility a delta refers to a reference
			// before the reference in packs inflated from thin packs,
			// so we need to check if it exists before blindly
			// setting it.
			// If it's been already been referenced, cache it.
			// Otherwise don't to save memory and only cache if
			// there are references to it.
			mu.Lock()
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
			mu.Unlock()
		case OBJ_REF_DELTA:
			log.Printf("Noting REF_DELTA to resolve: %v\n", ref)
			mu.Lock()
			o, ok := priorObjects[ref]
			if !ok {
				// It hasn't been seen yet, so just note
				// that there's a a delta against it for
				// later.
				// Since we haven't seen it yet, we don't
				// have a location.
				objCache := &packObject{
					oid:            sha1,
					deltasAgainst:  1,
					deltasResolved: 0,
				}
				priorObjects[ref] = objCache
			} else {
				o.deltasAgainst += 1
			}
			self := &packObject{
				idx:            i,
				oid:            sha1,
				location:       ObjectOffset(location),
				deltasAgainst:  0,
				deltasResolved: 0,
				typ:            t,
			}
			priorLocations[ObjectOffset(location)] = self
			*deltas = append(*deltas, self)
			mu.Unlock()
		case OBJ_OFS_DELTA:
			log.Printf("Noting OFS_DELTA to resolve from %v\n", location-int64(offset))
			mu.Lock()
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
				oid:            sha1,
				location:       ObjectOffset(location),
				deltasAgainst:  0,
				deltasResolved: 0,
				baselocation:   ObjectOffset(location) - ObjectOffset(offset),
				typ:            t,
			}
			priorLocations[ObjectOffset(location)] = self
			*deltas = append(*deltas, self)
			mu.Unlock()
		default:
			panic("Unhandled type in IndexPack: " + t.String())
		}
		return nil
	}
	crc32cb := func(i int, crc uint32) error {
		indexfile.CRC32[i] = crc
		return nil
	}

	return &indexfile, icb, cb, crc32cb, priorObjects, priorLocations
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
