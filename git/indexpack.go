package git

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
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
			rawdata = p.readEntryDataStream1(datareader)
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
	var p PackfileHeader
	var indexfile PackfileIndexV2

	iscopying := false
	var file *os.File
	if r2, ok := r.(*os.File); ok && !opts.Stdin {
		file = r2
	} else {
		pack, err := ioutil.TempFile(c.GitDir.File("objects/pack").String(), ".tmppackfileidx")
		if err != nil {
			return nil, err
		}
		defer pack.Close()
		file = pack
		r = io.TeeReader(r, pack)
		iscopying = true
		// If -stdin was specified, we copy it to the pack directory
		// namd after the trailer.
		defer func() {
			if rerr == nil && idx != nil {
				packhash, _ := indexfile.GetTrailer()
				base := fmt.Sprintf("%s/pack-%s", c.GitDir.File("objects/pack").String(), packhash)
				if err := os.Rename(pack.Name(), base+".pack"); err != nil {
					rerr = err
					return
				}
				fidx, err := os.Create(base + ".idx")
				if err != nil {
					rerr = err
					return
				}
				if err := indexfile.WriteIndex(fidx); err != nil {
					rerr = err
					return
				}
			}
		}()
	}
	if err := binary.Read(r, binary.BigEndian, &p); err != nil {
		return nil, err
	}

	if p.Signature != [4]byte{'P', 'A', 'C', 'K'} {
		return nil, fmt.Errorf("Invalid packfile: %+v", p.Signature)
	}
	if p.Version != 2 {
		return nil, fmt.Errorf("Unsupported packfile version: %d", p.Version)
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(int(p.Size))

	indexfile.magic = [4]byte{0377, 't', 'O', 'c'}
	indexfile.Version = 2

	indexfile.Sha1Table = make([]Sha1, p.Size)
	indexfile.CRC32 = make([]uint32, p.Size)
	indexfile.FourByteOffsets = make([]uint32, p.Size)
	priorObjects := make(map[Sha1]ObjectOffset)

	if iscopying {
		// Seek past the header that was just copied.
		file.Seek(12, io.SeekStart)
	}

	for i := uint32(0); i < p.Size; i += 1 {
		if opts.Verbose {
			progressF("Indexing objects: %2.f%% (%d/%d)", (float32(i+1) / float32(p.Size) * 100), i+1, p.Size)
		}
		location, err := file.Seek(0, io.SeekCurrent)
		if err != nil {
			panic(err)
		}

		checksum := crc32.NewIEEE()
		tr := io.TeeReader(r, checksum)
		t, _, ref, offset, _ := p.ReadHeaderSize(tr)

		var rawdata []byte

		if iscopying {
			// If we're copying from a reader, first we read from
			// the stream to have the data tee'd into the file,
			// then we read from the file because we need a
			// ReadSeeker in order to get both the compressed and
			// uncompressed data.
			br := &byteReader{tr, 0}
			rawdata = p.readEntryDataStream1(br)
		} else {
			// If we're reading from a file, we just read it
			// directly since we can seek back.
			rawdata, _ = p.readEntryDataStream2(file)
		}
		// The CRC32 checksum of the compressed data and the offset in
		// the file don't change regardless of type.
		mu.Lock()
		indexfile.CRC32[i] = checksum.Sum32()

		if location < (1 << 31) {
			indexfile.FourByteOffsets[i] = uint32(location)
		} else {
			indexfile.FourByteOffsets[i] = uint32(len(indexfile.EightByteOffsets)) | (1 << 31)
			indexfile.EightByteOffsets = append(indexfile.EightByteOffsets, uint64(location))
		}
		mu.Unlock()

		// The way we calculate the hash changes based on if it's a delta
		// or not.
		var sha1 Sha1
		switch t {
		case OBJ_COMMIT, OBJ_TREE, OBJ_BLOB, OBJ_TAG:
			sha1, _, err = HashSlice(t.String(), rawdata)
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
			priorObjects[sha1] = ObjectOffset(location)
			mu.Unlock()

			wg.Done()
		case OBJ_REF_DELTA:
			log.Printf("Resolving REF_DELTA from %v\n", ref)
			mu.Lock()
			o, ok := priorObjects[ref]
			if !ok {
				mu.Unlock()
				panic("Could not find basis for REF_DELTA")
			}
			// The refs in the index file need to be sorted in
			// order for GetObject to look up the other SHA1s
			// when resolving deltas. Chains don't have access
			// to the priorObjects map that we have here.
			sort.Sort(&indexfile)
			mu.Unlock()
			if err != nil {
				return indexfile, err
			}
			base, err := indexfile.getObjectAtOffset(file, int64(o), false)
			if err != nil {
				return indexfile, err
			}
			res := resolvedDelta{Value: base.GetContent()}
			_, val, err := calculateDelta(res, rawdata)
			if err != nil {
				return indexfile, err
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
				return nil, err
			}

			mu.Lock()
			priorObjects[sha1] = ObjectOffset(location)
			mu.Unlock()

			mu.Lock()
			for j := int(sha1[0]); j < 256; j++ {
				indexfile.Fanout[j]++
			}
			indexfile.Sha1Table[i] = sha1

			mu.Unlock()
			wg.Done()
		case OBJ_OFS_DELTA:
			log.Printf("Resolving OFS_DELTA from %v\n", location-int64(offset))
			base, err := indexfile.getObjectAtOffset(file, location-int64(offset), false)
			if err != nil {
				return indexfile, err
			}

			res := resolvedDelta{Value: base.GetContent()}
			_, val, err := calculateDelta(res, rawdata)
			if err != nil {
				return indexfile, err
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
				return nil, err
			}

			mu.Lock()
			priorObjects[sha1] = ObjectOffset(location)
			mu.Unlock()

			mu.Lock()
			for j := int(sha1[0]); j < 256; j++ {
				indexfile.Fanout[j]++
			}
			indexfile.Sha1Table[i] = sha1

			mu.Unlock()
			wg.Done()
		default:
			panic("Unhandled type in IndexPack: " + t.String())
		}
	}
	// Read the packfile trailer into the index trailer.
	binary.Read(r, binary.BigEndian, &indexfile.Packfile)
	sort.Sort(&indexfile)

	// The sorting may have changed things, so as a final pass, hash
	// everything to get the trailer (instead of doing it while we
	// were calculating everything.)
	err := indexfile.calculateTrailer()
	return indexfile, err
}

// Indexes the pack, and stores a copy in Client's .git/objects/pack directory as it's
// doing so. This is the equivalent of "git index-pack --stdin", but works with any
// reader.
func IndexAndCopyPack(c *Client, opts IndexPackOptions, r io.Reader) (PackfileIndex, error) {
	return IndexPack(c, opts, r)
}
