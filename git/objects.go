package git

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
)

var InvalidObject error = errors.New("Invalid object")

type GitObject interface {
	GetType() string
	GetContent() []byte
	GetSize() int

	// Print the object in human readable format.
	String() string
}

type GitBlobObject struct {
	size    int
	content []byte
}

func (GitBlobObject) GetType() string {
	return "blob"
}
func (b GitBlobObject) GetContent() []byte {
	if len(b.content) != b.size {
		panic(fmt.Sprintf("Content of blob does not match size. %d != %d", len(b.content), b.size))
	}
	return b.content
}
func (b GitBlobObject) GetSize() int {
	return b.size
}

func (b GitBlobObject) String() string {
	return string(b.content)
}

type GitCommitObject struct {
	size    int
	content []byte
}

func (c GitCommitObject) GetContent() []byte {
	return c.content
}

func (c GitCommitObject) GetType() string {
	return "commit"
}
func (c GitCommitObject) GetSize() int {
	return c.size
}
func (c GitCommitObject) String() string {
	return string(c.content)
}
func (c GitCommitObject) GetHeader(header string) string {
	headerPrefix := []byte(header + " ")
	reader := bytes.NewBuffer(c.content)
	for line, err := reader.ReadBytes('\n'); err == nil; line, err = reader.ReadBytes('\n') {
		if len(line) == 0 || len(line) == 1 {
			// EOF or just a '\n', separating the headers from the body
			return ""
		}
		if bytes.HasPrefix(line, headerPrefix) {
			line = bytes.TrimSuffix(line, []byte{'\n'})
			return string(bytes.TrimPrefix(line, headerPrefix))
		}
	}
	return ""
}

type GitTreeObject struct {
	size    int
	content []byte
}

func (t GitTreeObject) GetContent() []byte {
	return t.content
}

func (t GitTreeObject) GetType() string {
	return "tree"
}
func (t GitTreeObject) GetSize() int {
	return t.size
}
func (t GitTreeObject) String() string {
	// the raw format of content is
	// 	[permission] [name] \0 [20 bytes of Sha1]
	//
	// We need to convert this to human readable format, so we just
	// keep looking for nil bytes, and when we find one print the next 20
	// characters in hex and move i forward 20, recording where the next
	// name starts.
	//
	// The output format in human-readable format is supposed to be
	// [0 padded mode] [type] [sha1 printed in hex] [

	var nameStart int
	var ret string
	for i := 0; i < len(t.content); i++ {
		if t.content[i] == 0 {
			split := bytes.SplitN(t.content[nameStart:i], []byte{' '}, 2)
			perm := split[0]
			name := split[1]
			var gtype string
			if string(perm) == "40000" {
				// the only possible perms in a tree are 40000 (tree), 100644 (blob),
				// and 100755 (blob with mode +x, also a blob).. so we just check for
				// the only possible perm value for a tree, instead of looking up
				// the object in the .git/objects database.
				//
				// It's also supposed to be printed 0-padded with git cat-file -p,
				// so we just change the string since we're here.
				perm = []byte("040000")
				gtype = "tree"
			} else {
				gtype = "blob"
			}

			// Add when printing the value because i is currently set to the nil
			// byte.
			ret += fmt.Sprintf("%s %s %x\t%s\n", perm, gtype, t.content[i+1:i+21], name)
			i += 20
			nameStart = i + 1
		}
	}
	return ret
}

// Returns the byte array of a packed object from packfile, after
// resolving any deltas. (packfile should be the base name with no
// extension.)
func (c *Client) getPackedObject(packfile File, sha1 Sha1) (GitObject, error) {
	idx, err := (packfile + ".idx").Open()
	if err != nil {
		return nil, err
	}
	defer idx.Close()

	data, err := (packfile + ".pack").Open()
	if err != nil {
		return nil, err
	}
	defer data.Close()

	return getPackFileObject(idx, data, sha1)
}

func (c *Client) GetCommitObject(commit CommitID) (GitCommitObject, error) {
	o, err := c.GetObject(Sha1(commit))
	gco := o.(GitCommitObject)
	return gco, err
}
func (c *Client) GetObject(sha1 Sha1) (GitObject, error) {
	found, packfile, err := c.HaveObject(sha1)
	if err != nil {
		return nil, err
	}

	if found == false {
		return nil, fmt.Errorf("Object not found.")
	}

	var b []byte
	if packfile != "" {
		return c.getPackedObject(packfile, sha1)
	} else {
		objectname := fmt.Sprintf("%s/objects/%x/%x", c.GitDir, sha1[0:1], sha1[1:])
		f, err := os.Open(objectname)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		uncompressed, err := zlib.NewReader(f)
		if err != nil {
			return nil, err
		}
		b, err = ioutil.ReadAll(uncompressed)
		if err != nil {
			return nil, err
		}

	}

	if strings.HasPrefix(string(b), "blob ") {
		var size int
		var content []byte
		for idx, val := range b {
			if val == 0 {
				content = b[idx+1:]
				if size, err = strconv.Atoi(string(b[5:idx])); err != nil {
					fmt.Printf("Error converting % x to int at idx: %d", b[5:idx], idx)
				}
				break
			}
		}
		return GitBlobObject{size, content}, nil
	} else if strings.HasPrefix(string(b), "commit ") {
		var size int
		var content []byte
		for idx, val := range b {
			if val == 0 {
				content = b[idx+1:]
				if size, err = strconv.Atoi(string(b[7:idx])); err != nil {
					fmt.Printf("Error converting % x to int at idx: %d", b[7:idx], idx)
				}
				break
			}
		}
		return GitCommitObject{size, content}, nil
	} else if strings.HasPrefix(string(b), "tree ") {
		var size int
		var content []byte
		for idx, val := range b {
			if val == 0 {
				content = b[idx+1:]
				if size, err = strconv.Atoi(string(b[5:idx])); err != nil {
					fmt.Printf("Error converting % x to int at idx: %d", b[5:idx], idx)
				}
				break
			}
		}
		return GitTreeObject{size, content}, nil
	} else {
		fmt.Printf("Content: %s\n", string(b))
	}
	return nil, InvalidObject
}
