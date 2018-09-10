package git

import (
	"bufio"
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
	if b.content == nil {
		panic("Attempted to get content but only loaded metadata")
	}
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
	if c.content == nil {
		panic("Attempted to get content but only loaded metadata")
	}
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
	if t.content == nil {
		panic("Attempted to get content but only loaded metadata")
	}
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
func (c *Client) getPackedObject(packfile File, sha1 Sha1, metaOnly bool) (GitObject, error) {
	cacheloc, cached := c.objectCache[sha1]
	if !cached {
		panic("Attempt to use pack file before parsing index.")
	}

	f, err := os.Open((cacheloc.packfile + ".pack").String())
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return cacheloc.index.getObjectAtOffset(f, cacheloc.offset, metaOnly)
}

func (c *Client) GetCommitObject(commit CommitID) (GitCommitObject, error) {
	o, err := c.GetObject(Sha1(commit))
	gco, ok := o.(GitCommitObject)
	if ok {
		return gco, nil
	}
	return gco, fmt.Errorf("Could not convert object %v to commit object: %v", o, err)
}
func (c *Client) GetObjectMetadata(sha1 Sha1) (string, uint64, error) {
	obj, err := c.getObject(sha1, true)
	if err != nil {
		return "", 0, err
	}
	return obj.GetType(), uint64(obj.GetSize()), nil
}

func (c *Client) GetObject(sha1 Sha1) (GitObject, error) {
	return c.getObject(sha1, false)
}

func (c *Client) getObject(sha1 Sha1, metaOnly bool) (GitObject, error) {
	if gobj, ok := c.objcache[shaRef{sha1, metaOnly}]; ok {
		// FIXME: We should determine why this is attempting to retrieve the
		// same things multiple times and fix the source.
		return gobj, nil
	}
	found, packfile, err := c.HaveObject(sha1)
	if err != nil {
		return nil, err
	}

	if found == false {
		return nil, fmt.Errorf("Object not found.")
	}

	var b []byte
	if packfile != "" {
		gobj, err := c.getPackedObject(packfile, sha1, metaOnly)
		if err != nil {
			return nil, err
		}
		c.objcache[shaRef{sha1, metaOnly}] = gobj
		return gobj, nil
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
		if metaOnly {
			buf := bufio.NewReader(uncompressed)
			line, err := buf.ReadBytes(0)

			pieces := strings.Fields(string(bytes.TrimSuffix(line, []byte{0})))
			if len(pieces) != 2 {
				return nil, fmt.Errorf("Invalid object")
			}
			sz, err := strconv.Atoi(pieces[1])
			if err != nil {
				return nil, fmt.Errorf("Invalid size: %v", err)
			}
			switch pieces[0] {
			case "blob":
				return GitBlobObject{sz, nil}, nil
			case "tree":
				return GitTreeObject{sz, nil}, nil
			case "commit":
				return GitCommitObject{sz, nil}, nil
			}
			return nil, fmt.Errorf("Unknown object type: %v", pieces[0])
		} else {
			b, err = ioutil.ReadAll(uncompressed)

			if err != nil {
				return nil, err
			}
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
		gobj := GitBlobObject{size, content}
		c.objcache[shaRef{sha1, metaOnly}] = gobj
		return gobj, nil
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
		gobj := GitCommitObject{size, content}
		c.objcache[shaRef{sha1, metaOnly}] = gobj
		return gobj, nil
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
		gobj := GitTreeObject{size, content}
		c.objcache[shaRef{sha1, metaOnly}] = gobj
		return gobj, nil
	} else {
		fmt.Printf("Content: %s\n", string(b))
	}
	return nil, InvalidObject
}
