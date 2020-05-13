package git

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
)

func HashReaderWithSize(t string, sz int64, r io.Reader) (Sha1, error) {
	h := sha1.New()
	fmt.Fprintf(h, "%s %d\000", t, sz)

	if sz < 0 {
		return Sha1{}, fmt.Errorf("Invalid size: %v", sz)
	} else if sz == 0 {
		return Sha1FromSlice(h.Sum(nil))
	}
	n, err := io.Copy(h, r)
	if err != nil {
		return Sha1{}, err
	}
	if n != sz {
		return Sha1{}, fmt.Errorf("Unexpected reader size (%v != %v)", n, sz)
	}
	return Sha1FromSlice(h.Sum(nil))
}

// Hashes the data of r with object type t, and returns
// the hash, and the data that was read from r.
func HashReader(t string, r io.Reader) (Sha1, []byte, error) {
	// Need to read the whole reader in order to find the size
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return Sha1{}, nil, err
	}

	h := sha1.New()
	fmt.Fprintf(h, "%s %d\000%s", t, len(data), data)
	s, err := Sha1FromSlice(h.Sum(nil))
	return s, data, err
}

func HashSlice(t string, data []byte) (Sha1, []byte, error) {
	r := bytes.NewReader(data)
	return HashReader(t, r)
}

func HashFile(t, filename string) (Sha1, []byte, error) {
	if File(filename).IsSymlink() {
		l, err := os.Readlink(filename)
		if err != nil {
			return Sha1{}, nil, err
		}
		return HashReader(t, strings.NewReader(l))
	} else {
		r, err := os.Open(filename)
		if err != nil {
			return Sha1{}, nil, err
		}
		defer r.Close()
		return HashReader(t, r)
	}
}
