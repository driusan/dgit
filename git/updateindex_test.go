package git

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"testing"
)

func compareIndex(got, want Index) bool {

	if len(got.Objects) != len(want.Objects) {
		return false
	}
	cmpObj := func(got, want IndexEntry) bool {
		return got.PathName == want.PathName &&
			got.Sha1 == want.Sha1 &&
			got.Mode == want.Mode
	}
	for i := range want.Objects {
		if !cmpObj(*got.Objects[i], *want.Objects[i]) {
			return false
		}
	}
	return true
}

func TestUpdateIndex(t *testing.T) {
	testcases := []struct {
		Index       Index
		Options     UpdateIndexOptions
		Files       []File
		Expected    Index
		ExpectedErr bool
	}{
		{
			// Add a known file without --add, should be an error
			Index{},
			UpdateIndexOptions{},
			[]File{File("test/bar")},
			Index{
				Objects: []*IndexEntry{
					&IndexEntry{PathName: "foo"},
				},
			},
			true,
		},
		{
			// Add a known file with --add, should succeed
			Index{},
			UpdateIndexOptions{
				Add: true,
			},
			[]File{File("foo")},
			Index{
				Objects: []*IndexEntry{
					&IndexEntry{
						FixedIndexEntry: FixedIndexEntry{
							Mode: ModeBlob,
							Sha1: hashString("foo"),
						},
						PathName: "foo",
					},
				},
			},
			false,
		},
		{
			// Even with --add, trying to add a directory should fail
			Index{},
			UpdateIndexOptions{Add: true},
			[]File{File("test")},
			Index{},
			true,
		},
		// Remove a file without --remove, should fail
		{
			Index{
				Objects: []*IndexEntry{
					&IndexEntry{
						FixedIndexEntry: FixedIndexEntry{
							Mode: ModeBlob,
							Sha1: hashString("foo"),
						},
						PathName: "foob",
					},
				},
			},
			UpdateIndexOptions{},
			[]File{File("foob")},
			Index{},
			true,
		},
		// Remove a file with --remove, should succeed
		{
			Index{
				Objects: []*IndexEntry{
					&IndexEntry{
						FixedIndexEntry: FixedIndexEntry{
							Mode: ModeBlob,
							Sha1: hashString("foo"),
						},
						PathName: "foob",
					},
				},
			},
			UpdateIndexOptions{Remove: true},
			[]File{File("foob")},
			Index{},
			false,
		},
	}

	// Set up a temporary directory, and put 2 known files in it, so that
	// our tests can run from a deterministic state.
	wd, err := ioutil.TempDir("", "gitupdateindextest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(wd)
	os.Chdir(wd)

	// FIXME: This should be git.Init(), but Init is currently in cmd.Init..
	c, err := Init(nil, InitOptions{Quiet: true}, "")
	if err != nil {
		t.Fatal(err)
	}

	// Setup a working tree.
	if err := os.Mkdir(wd+"/test", 0755); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(wd+"/foo", []byte("foo"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(wd+"/test/bar", []byte("bar"), 0755); err != nil {
		t.Fatal(err)
	}

	for i, tc := range testcases {
		newidx, err := UpdateIndex(c, &tc.Index, tc.Options, tc.Files)
		if tc.ExpectedErr {
			if err == nil {
				t.Errorf("Case %d: expected error, got none", i)
			}
			continue
		} else {
			if err != nil {
				t.Errorf("Case %d: got error %v expected none", i, err)
				continue
			}
		}

		if !compareIndex(*newidx, tc.Expected) {
			t.Errorf("Case %d: got %v want %v", i, newidx, tc.Expected)
		}
	}
}

// Tests adding symlinks to the index with UpdateIndex
func TestUpdateIndexSymlinks(t *testing.T) {
	// Set up a temporary directory, and put 2 known files in it, so that
	// our tests can run from a deterministic state.
	wd, err := ioutil.TempDir("", "gitupdateindextest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(wd)
	os.Chdir(wd)

	c, err := Init(nil, InitOptions{Quiet: true}, "")
	if err != nil {
		t.Fatal(err)
	}
	// Setup a working tree.
	if err := ioutil.WriteFile(wd+"/foo", []byte("foo content"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("foo", "works"); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink("ohnoes", "broken"); err != nil {
		t.Fatal(err)
	}

	newidx, err := UpdateIndex(c, &Index{}, UpdateIndexOptions{Add: true}, []File{"works", "broken"})
	if err != nil {
		t.Fatal(err)
	}
	if len(newidx.Objects) != 2 {
		t.Fatalf("Unexpected number of entries in index: got %v want 2", len(newidx.Objects))
	}
	newobjects := newidx.Objects
	// Content for "broken". Value comes from what real git adds.
	sha, err := Sha1FromString("42dfea9398d23b9d95bdb97624a7cfc117ac1091")
	if err != nil {
		t.Fatal(err)
	}
	if newobjects[0].Sha1 != sha {
		t.Errorf("Unexpected hash: got %v want %v", newobjects[0].Sha1, sha)
	}
	if newobjects[0].Mode != ModeSymlink {
		t.Errorf("Unexpected file mode. got %v want %v", newobjects[0].Mode, ModeSymlink)
	}

	// content for "works". Value comes from what real git adds.
	sha, err = Sha1FromString("19102815663d23f8b75a47e7a01965dcdc96468c")
	if err != nil {
		t.Fatal(err)
	}
	if newobjects[1].Sha1 != sha {
		t.Errorf("Unexpected hash: got %v want %v", newobjects[1].Sha1, sha)
	}
	if newobjects[1].Mode != ModeSymlink {
		t.Errorf("Unexpected file mode. got %v want %v", newobjects[1].Mode, ModeSymlink)
	}
}

func TestUpdateIndexIndexInfo(t *testing.T) {
	// Set up a temporary directory, and put 2 known files in it, so that
	// our tests can run from a deterministic state.
	wd, err := ioutil.TempDir("", "gitupdateindextest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(wd)
	os.Chdir(wd)

	c, err := Init(nil, InitOptions{Quiet: true}, "")

	info := strings.NewReader(`100644 blob 1000000000000000000000000000000000000000	dir/file1
100644 blob 2000000000000000000000000000000000000000	dir/file2
100644 blob 3000000000000000000000000000000000000000	dir/file3
100644 blob 4000000000000000000000000000000000000000	dir/file4
100644 blob 5000000000000000000000000000000000000000	dir/file5
`)
	newidx, err := UpdateIndex(c, &Index{}, UpdateIndexOptions{IndexInfo: info}, []File{"works", "broken"})
	if err != nil {
		t.Fatal(err)
	}
	if newidx == nil {
		t.Fatal("No index returned")

	}
	if len(newidx.Objects) != 5 {
		t.Fatalf("Incorrect number of objects in new index: got %v want 5", len(newidx.Objects))
	}

	for i, o := range newidx.Objects {
		if o.Mode != ModeBlob {
			t.Errorf("Unexpected mode for index %d: got %v want %v", i, o.Mode, ModeBlob)
		}
		if expected := IndexPath(fmt.Sprintf("dir/file%d", i+1)); expected != o.PathName {
			t.Errorf("Unexpected path for index %d: got %v want %v", i, o.PathName, expected)
		}
		shastr := fmt.Sprintf("%d000000000000000000000000000000000000000", i+1)
		if shastr != o.Sha1.String() {
			t.Errorf("Unexpected sha1 for index %d: got %v want %v", i, o.Sha1, shastr)
		}
	}
}
