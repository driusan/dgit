package git

import (
	"io/ioutil"
	"os"
	"testing"
)

// TestSimpleCommits tests that an initial commit works,
// and that a second commit using it as a parent works.
// It also tests that the second commit can be done while
// in a detached head mode.
func TestSimpleCommits(t *testing.T) {
	dir, err := ioutil.TempDir("", "gitcommit")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Init a repo to test an initial commit in.
	c, err := Init(nil, InitOptions{Quiet: true}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(dir+"/foo.txt", []byte("foo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Add(c, AddOptions{}, []File{"foo.txt"}); err != nil {
		t.Fatal(err)
	}

	// Set the environment variables used by CommitTree to a known value,
	// to ensure repeatable tests.
	if err := os.Setenv("GIT_COMMITTER_NAME", "John Smith"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("GIT_COMMITTER_EMAIL", "test@example.com"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("GIT_AUTHOR_NAME", "John Smith"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("GIT_AUTHOR_EMAIL", "test@example.com"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("GIT_COMMITTER_DATE", "Mon, 02 Jan 2006 15:04:05 -0700"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("GIT_AUTHOR_DATE", "Mon, 02 Jan 2006 15:04:05 -0700"); err != nil {
		t.Fatal(err)
	}

	initialCmt, err := Commit(c, CommitOptions{}, "Initial commit", nil)
	if err != nil {
		t.Fatal(err)
	}

	expected, err := CommitIDFromString("02831f447cd60fc8288dc630e135e240d4369666")
	if err != nil {
		t.Fatal(err)
	}
	if initialCmt != expected {
		t.Errorf("Unexpected hash for expected commit. got %v want %v", initialCmt, expected)
	}
	if err := os.Setenv("GIT_AUTHOR_DATE", "Mon, 02 Jan 2007 15:54:05 -0700"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("GIT_COMMITTER_DATE", "Mon, 02 Jan 2007 15:54:05 -0700"); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(dir+"/foo.txt", []byte("bar\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Add(c, AddOptions{}, []File{"foo.txt"}); err != nil {
		t.Fatal(err)
	}
	cid, err := Commit(c, CommitOptions{}, "Changed foo to bar", nil)
	if err != nil {
		t.Fatal(err)
	}
	expected, err = CommitIDFromString("f379502e6874625280c5a51cfa916af0b3e968b5")
	if err != nil {
		t.Fatal(err)
	}
	if cid != expected {
		t.Errorf("Unexpected hash for second commit. got %v want %v", cid, expected)
	}

	// Now go back to the first commit in a detached head state, and try
	// again.

	if err := CheckoutCommit(c, CheckoutOptions{}, initialCmt); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(dir+"/foo.txt", []byte("bar\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Add(c, AddOptions{}, []File{"foo.txt"}); err != nil {
		t.Fatal(err)
	}
	cid, err = Commit(c, CommitOptions{}, "Changed foo to bar", nil)
	if err != nil {
		t.Fatal("Commit with detached head error:", err)
	}
	if cid != expected {
		t.Errorf("Unexpected hash for second commit while in detached head mode. got %v want %v", cid, expected)
	}
}

// TestUnmergedCommit tests that Commit() produces an
// error when there are merge entries in the index.
func TestUnmergedCommit(t *testing.T) {
	gitdir, err := ioutil.TempDir("", "gitcommitunmerged")
	if err != nil {
		t.Fatal(err)
	}
	//	defer os.RemoveAll(gitdir)

	c, err := Init(nil, InitOptions{Quiet: true}, gitdir)
	if err != nil {
		t.Fatal(err)
	}
	idx := NewIndex()
	idx.Objects = []*IndexEntry{
		&IndexEntry{
			PathName: IndexPath("foo"),
			FixedIndexEntry: FixedIndexEntry{
				Mode:  ModeBlob,
				Fsize: 4,
				Sha1:  hashString("bar\n"),
				// 3 == len(filename)
				Flags: uint16(Stage1)<<12 | 3,
			},
		},
		&IndexEntry{
			PathName: IndexPath("foo"),
			FixedIndexEntry: FixedIndexEntry{
				Mode:  ModeBlob,
				Fsize: 4,
				Sha1:  hashString("baz\n"),
				// 3 == len(filename)
				Flags: uint16(Stage2)<<12 | 3,
			},
		},
	}
	// Make sure "bar\n" and "baz\n" exist, so we're testing the right
	// thing
	if _, err := c.WriteObject("blob", []byte("bar\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := c.WriteObject("blob", []byte("baz\n")); err != nil {
		t.Fatal(err)
	}

	// Write the index
	f, err := c.GitDir.Create("index")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := idx.WriteIndex(f); err != nil {
		t.Fatal(err)
	}
	// Sanity check: make sure the index got written.
	if files, err := LsFiles(c, LsFilesOptions{Stage: true, Cached: true}, nil); len(files) != 2 || err != nil {
		t.Fatal("Did not correctly write index", len(files), err, c)
	}

	// Finally, do the test..
	_, err = Commit(c, CommitOptions{}, "I am a test", nil)
	if err == nil {
		t.Error("Was able to commit an index with unresolved conflicts.")
	}
}
