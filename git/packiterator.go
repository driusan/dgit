package git

import (
	"bufio"
	"compress/flate"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sync"

	"encoding/binary"
)

type byteCounter struct {
	io.Reader
	n int64
}

func (r *byteCounter) Read(buf []byte) (int, error) {
	n, err := r.Reader.Read(buf)
	r.n += int64(n)
	return n, err
}

type flateCounter struct {
	flate.Reader
	n int64
}

func (r *flateCounter) Read(buf []byte) (int, error) {
	n, err := r.Reader.Read(buf)
	r.n += int64(n)
	return n, err
}

func (r *flateCounter) ReadByte() (byte, error) {
	r.n += 1
	return r.Reader.ReadByte()
}

type packIterator func(r io.ReaderAt, i, n int, loc int64, compsz int64, t PackEntryType, osz PackEntrySize, deltaref Sha1, deltaoffset ObjectOffset, rawdata []byte) error

func iteratePack(c *Client, r io.Reader, initcallback func(int), callback packIterator, trailerCB func(packtrailer Sha1) error) (*os.File, error) {
	// if the reader is not a file, tee it into a temp file to resolve
	// deltas from.
	var pack *os.File

	counter := &byteCounter{r, 0}
	if f, ok := r.(*os.File); ok && f != os.Stdin {
		pack = f
		r = counter
	} else {
		var err error
		pack, err = ioutil.TempFile(c.GitDir.File("objects/pack").String(), ".tmppackfile")
		if err != nil {
			return nil, err
		}
		// Do not defer pack.Close, it's the caller's responsibility to close it.

		// Only tee into the pack file if it's not a file
		r = io.TeeReader(counter, pack)
	}

	var p PackfileHeader
	if err := binary.Read(r, binary.BigEndian, &p); err != nil {
		return nil, err
	}

	if p.Signature != [4]byte{'P', 'A', 'C', 'K'} {
		return nil, fmt.Errorf("Invalid packfile: %+v", p.Signature)
	}
	if p.Version != 2 {
		return nil, fmt.Errorf("Unsupported packfile version: %d", p.Version)
	}

	loc := counter.n
	br := bufio.NewReader(r)
	initcallback(int(p.Size))

	var wg sync.WaitGroup
	for i := uint32(0); i < p.Size; i += 1 {
		wg.Add(1)
		t, sz, deltasha, deltaoff, rawheader := p.ReadHeaderSize(br)

		datacounter := flateCounter{br, 0}
		raw := p.readEntryDataStream1(&datacounter)

		compsize := int64(len(rawheader)) + int64(datacounter.n)

		// Doing this in goroutines seems to crash 9front and provides very
		// little performance gain, so for now only do 1 a time.
		func(i int, psize int, loc int64, compsize int64, t PackEntryType, sz PackEntrySize, deltasha Sha1, deltaoff ObjectOffset, raw []byte) {
		//go func(i int, psize int, loc int64, compsize int64, t PackEntryType, sz PackEntrySize, deltasha Sha1, deltaoff ObjectOffset, raw []byte) {
			defer wg.Done()
			if err := callback(pack, i, psize, loc, compsize, t, sz, deltasha, deltaoff, raw); err != nil {
				panic(err)
			}
		}(int(i), int(p.Size), loc, compsize, t, sz, deltasha, deltaoff, raw)

		loc += compsize
	}

	wg.Wait()
	// We need to read the packfile trailer so that it gets tee'd into
	// the temp file, or it won't be there for index-pack.
	var trailer PackfileIndexV2
	if err := binary.Read(br, binary.BigEndian, &trailer.Packfile); err != nil {
		return nil, err
	}
	trailerCB(trailer.Packfile)

	return pack, nil
}
