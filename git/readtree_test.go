package git

import (
	//	"fmt"
	"io/ioutil"
	"os"
	"testing"
)

// Tests that ReadTree creates Stage1, Stage2, and Stage3 when
// they can't be resolved.
func TestReadTreeThreeWayConflict(t *testing.T) {
	gitdir, err := ioutil.TempDir("", "gitreadtree")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(gitdir)
	os.Chdir(gitdir)

	c, err := Init(nil, InitOptions{Quiet: true}, gitdir)
	if err != nil {
		t.Fatal(err)
	}
	index := NewIndex()
	if err := ioutil.WriteFile(c.WorkDir.String()+"/foo", []byte("foo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	index, err = UpdateIndex(c, index, UpdateIndexOptions{Add: true}, []File{"foo"})
	if err != nil {
		t.Fatal(err)
	}
	foo, err := writeTree(c, "", index.Objects)
	if err != nil {
		t.Fatal(err)
	}

	if err := ioutil.WriteFile(c.WorkDir.String()+"/foo", []byte("bar\n"), 0644); err != nil {
		t.Fatal(err)
	}
	index, err = UpdateIndex(c, index, UpdateIndexOptions{Add: true}, []File{"foo"})
	if err != nil {
		t.Fatal(err)
	}
	bar, err := writeTree(c, "", index.Objects)

	if err := ioutil.WriteFile(c.WorkDir.String()+"/foo", []byte("baz\n"), 0644); err != nil {
		t.Fatal(err)
	}

	index, err = UpdateIndex(c, index, UpdateIndexOptions{Add: true}, []File{"foo"})
	if err != nil {
		t.Fatal(err)
	}
	baz, err := writeTree(c, "", index.Objects)
	if err := os.Remove(c.WorkDir.String() + "/foo"); err != nil {
		t.Fatal(err)
	}
	index, err = UpdateIndex(c, index, UpdateIndexOptions{Remove: true}, []File{"foo"})
	if err != nil {
		t.Fatal(err)
	}
	nofoo, err := writeTree(c, "", index.Objects)
	if err != nil {
		t.Fatal(err)
	}

	// Case 1: base, ours, and theirs are all modified in different ways.
	// There should be 2 stages.
	idx, err := ReadTreeThreeWay(c, ReadTreeOptions{Reset: true}, TreeID(foo), TreeID(bar), TreeID(baz))
	if err != nil {
		t.Errorf("ReadTree error: %v", err)
	}

	if idx == nil {
		t.Fatalf("Got nil index from read-tree")
	}
	if len(idx.Objects) != 3 {
		t.Fatalf("Unexpected number of stages in tree. Got %v want %v", len(idx.Objects), 3)
	}

	if idx.Objects[0].Stage() != Stage1 {
		t.Errorf("Unexpected stage for first object: got %v want %v", idx.Objects[0].Stage(), Stage1)
	}
	if idx.Objects[0].PathName != IndexPath("foo") {
		t.Errorf("Unexpected file for object: got %v want %v", idx.Objects[0].PathName, IndexPath("foo"))
	}
	if hash := hashString("foo\n"); idx.Objects[0].Sha1 != hash {
		t.Errorf("Unexpected hash for foo: got %v want %v", idx.Objects[0].Sha1, hash)
	}

	if idx.Objects[1].Stage() != Stage2 {
		t.Errorf("Unexpected stage for first object: got %v want %v", idx.Objects[1].Stage(), Stage2)
	}
	if idx.Objects[1].PathName != IndexPath("foo") {
		t.Errorf("Unexpected file for object: got %v want %v", idx.Objects[1].PathName, IndexPath("foo"))
	}
	if hash := hashString("bar\n"); idx.Objects[1].Sha1 != hash {
		t.Errorf("Unexpected hash for foo: got %v want %v", idx.Objects[1].Sha1, hash)
	}

	if idx.Objects[2].Stage() != Stage3 {
		t.Errorf("Unexpected stage for first object: got %v want %v", idx.Objects[2].Stage(), Stage3)
	}
	if idx.Objects[2].PathName != IndexPath("foo") {
		t.Errorf("Unexpected file for object: got %v want %v", idx.Objects[2].PathName, IndexPath("foo"))
	}
	if hash := hashString("baz\n"); idx.Objects[2].Sha1 != hash {
		t.Errorf("Unexpected hash for foo: got %v want %v", idx.Objects[2].Sha1, hash)
	}

	// Case 2: Base and ours are modified, "theirs" is missing.
	_, err = ReadTree(c, ReadTreeOptions{Empty: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	idx, err = ReadTreeThreeWay(c, ReadTreeOptions{Reset: true}, TreeID(foo), TreeID(bar), TreeID(nofoo))
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Objects) != 2 {
		t.Fatalf("Unexpected number of stages in tree. Got %v want %v", len(idx.Objects), 2)
	}
	if idx.Objects[0].Stage() != Stage1 {
		t.Errorf("Unexpected stage for first object: got %v want %v", idx.Objects[0].Stage(), Stage1)
	}
	if idx.Objects[0].PathName != IndexPath("foo") {
		t.Errorf("Unexpected file for object: got %v want %v", idx.Objects[0].PathName, IndexPath("foo"))
	}
	if hash := hashString("foo\n"); idx.Objects[0].Sha1 != hash {
		t.Errorf("Unexpected hash for foo: got %v want %v", idx.Objects[0].Sha1, hash)
	}

	if idx.Objects[1].Stage() != Stage2 {
		t.Errorf("Unexpected stage for first object: got %v want %v", idx.Objects[1].Stage(), Stage2)
	}
	if idx.Objects[1].PathName != IndexPath("foo") {
		t.Errorf("Unexpected file for object: got %v want %v", idx.Objects[1].PathName, IndexPath("foo"))
	}
	if hash := hashString("bar\n"); idx.Objects[1].Sha1 != hash {
		t.Errorf("Unexpected hash for foo: got %v want %v", idx.Objects[1].Sha1, hash)
	}

	// Case 3: base and theirs are modified, ours is missing.
	_, err = ReadTree(c, ReadTreeOptions{Empty: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	idx, err = ReadTreeThreeWay(c, ReadTreeOptions{Reset: true}, TreeID(foo), TreeID(nofoo), TreeID(baz))
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Objects) != 2 {
		t.Fatalf("Unexpected number of stages in tree. Got %v want %v", len(idx.Objects), 2)
	}

	if idx.Objects[0].Stage() != Stage1 {
		t.Errorf("Unexpected stage for first object: got %v want %v", idx.Objects[0].Stage(), Stage1)
	}
	if idx.Objects[0].PathName != IndexPath("foo") {
		t.Errorf("Unexpected file for object: got %v want %v", idx.Objects[0].PathName, IndexPath("foo"))
	}
	if hash := hashString("foo\n"); idx.Objects[0].Sha1 != hash {
		t.Errorf("Unexpected hash for foo: got %v want %v", idx.Objects[0].Sha1, hash)
	}

	if idx.Objects[1].Stage() != Stage3 {
		t.Errorf("Unexpected stage for first object: got %v want %v", idx.Objects[1].Stage(), Stage3)
	}
	if idx.Objects[1].PathName != IndexPath("foo") {
		t.Errorf("Unexpected file for object: got %v want %v", idx.Objects[1].PathName, IndexPath("foo"))
	}
	if hash := hashString("baz\n"); idx.Objects[1].Sha1 != hash {
		t.Errorf("Unexpected hash for foo: got %v want %v", idx.Objects[1].Sha1, hash)
	}

	// Case 4: missing from base, ours and theirs added different files.
	_, err = ReadTree(c, ReadTreeOptions{Empty: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	idx, err = ReadTreeThreeWay(c, ReadTreeOptions{Reset: true}, TreeID(nofoo), TreeID(bar), TreeID(baz))
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Objects) != 2 {
		t.Fatalf("Unexpected number of stages in tree. Got %v want %v", len(idx.Objects), 2)
	}
	if idx.Objects[0].Stage() != Stage2 {
		t.Errorf("Unexpected stage for first object: got %v want %v", idx.Objects[0].Stage(), Stage2)
	}
	if idx.Objects[0].PathName != IndexPath("foo") {
		t.Errorf("Unexpected file for object: got %v want %v", idx.Objects[0].PathName, IndexPath("foo"))
	}
	if hash := hashString("bar\n"); idx.Objects[0].Sha1 != hash {
		t.Errorf("Unexpected hash for foo: got %v want %v", idx.Objects[0].Sha1, hash)
	}

	if idx.Objects[1].Stage() != Stage3 {
		t.Errorf("Unexpected stage for first object: got %v want %v", idx.Objects[1].Stage(), Stage3)
	}
	if idx.Objects[1].PathName != IndexPath("foo") {
		t.Errorf("Unexpected file for object: got %v want %v", idx.Objects[1].PathName, IndexPath("foo"))
	}
	if hash := hashString("baz\n"); idx.Objects[1].Sha1 != hash {
		t.Errorf("Unexpected hash for foo: got %v want %v", idx.Objects[1].Sha1, hash)
	}
}
