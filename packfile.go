package main

import (
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

type packfileSignature struct {
	Signature [4]byte
	Version   uint32
	Size      uint32
}

func unpack(r io.ReadSeeker) {
	var p packfileSignature
	binary.Read(r, binary.BigEndian, &p)
	if p.Signature != [4]byte{'P', 'A', 'C', 'K'} {
		return //err
	}
	if p.Version != 2 {
		return //err
	}
	b := make([]byte, 1)
	var i int
	r.Read(b)
	for b[0] >= 128 {
		r.Read(b)
		i += 1
	}
	//r.Seek(2, 1)
	zr, err := zlib.NewReader(r)
	if err != nil {
		panic(err)
	}
	defer zr.Close()
	io.Copy(os.Stderr, zr)

}
