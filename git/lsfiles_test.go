package git

import (
	"io/ioutil"
	"os"

	"testing"
)

func TestLsFiles(t *testing.T) {
	testcases := []struct {
		Index    Index
		Options  LsFilesOptions
		Pwd      File
		FilesArg []File
		Expected []File
	}{
		// --cached test
		{
			// LsFiles from root with nothing in the index
			Index{},
			LsFilesOptions{Cached: true},
			"",
			nil,
			nil,
		},

		{
			// LsFiles from root with nothing in the index
			Index{},
			LsFilesOptions{Cached: true},
			"",
			nil,
			nil,
		},

		// --other test cases:
		{
			// Base case from root of repo
			Index{},
			LsFilesOptions{Others: true},
			"",
			nil,
			[]File{"anothertest/bar", "foo", "test/bar"},
		},
		{
			// LsFiles within a subdirectory
			Index{},
			LsFilesOptions{Others: true},
			"test",
			nil,
			[]File{"bar"},
		},
		{
			// LsFiles with explicit ".." parameter within a subdirectory
			Index{},
			LsFilesOptions{Others: true},
			"test",
			[]File{".."},
			[]File{"../anothertest/bar", "../foo", "bar"},
		},
		{
			// LsFiles with explicit absolute path from subdirectory.
			Index{},
			LsFilesOptions{Others: true},
			"test",
			[]File{"$absolutepath$"},
			[]File{"../anothertest/bar", "../foo", "bar"},
		},
		{
			// LsFiles with explicit relative path from root
			Index{},
			LsFilesOptions{Others: true},
			"",
			[]File{"anothertest"},
			[]File{"anothertest/bar"},
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
	if err := os.Mkdir(wd+"/anothertest", 0755); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(wd+"/foo", []byte("foo"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(wd+"/test/bar", []byte("bar"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(wd+"/anothertest/bar", []byte("bar"), 0755); err != nil {
		t.Fatal(err)
	}

	c, err := NewClient(wd+"/.git", wd)
	if err != nil {
		t.Fatal(err)
	}
	for i, tc := range testcases {
		if err := os.Chdir(wd + "/" + tc.Pwd.String()); err != nil {
			t.Fatal(err)
		}
		idx, err := c.GitDir.Create("index")
		tc.Index.fixedGitIndex.Signature = [4]byte{'D', 'I', 'R', 'C'}
		tc.Index.fixedGitIndex.Version = 2
		if err != nil {
			t.Fatal(err)
		}
		if err := tc.Index.WriteIndex(idx); err != nil {
			idx.Close()
			t.Fatal(err)
		}
		idx.Close()

		for i, file := range tc.FilesArg {
			if file == "$absolutepath$" {
				tc.FilesArg[i] = File(wd)
			}
		}
		indexes, err := LsFiles(c, tc.Options, tc.FilesArg)
		if err != nil {
			t.Errorf("Case %d: got error %v expected none", i, err)
		}
		if len(indexes) != len(tc.Expected) {
			t.Errorf("Case %d: Unexpected number of results: got %v want %v", i, indexes, tc.Expected)
			continue
		}
		for j, idx := range indexes {
			file, err := idx.PathName.FilePath(c)
			if err != nil {
				t.Errorf("Case %d index %d: %v", i, j, err)
			}
			if file != tc.Expected[j] {
				t.Errorf("Case %d index %d: got %v want %v", i, j, file, tc.Expected[j])
			}
		}

	}
}
