package git

import (
	"crypto/sha1"
	"fmt"
	"io"
	"io/ioutil"
	"os"
)

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

func HashFile(t, filename string) (Sha1, []byte, error) {
	r, err := os.Open(filename)
	if err != nil {
		return Sha1{}, nil, err
	}
	defer r.Close()
	return HashReader(t, r)
}
