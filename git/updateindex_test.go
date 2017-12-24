package git

import (
	"io/ioutil"
	"os"

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
	os.Chdir(wd)

	// FIXME: This should be git.Init(), but Init is currently in cmd.Init..
	os.Mkdir(wd+"/.git", 0755)
	defer os.RemoveAll(wd)

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

	c, err := NewClient(wd+"/.git", wd)
	if err != nil {
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
