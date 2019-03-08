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

	expected, err := CommitIDFromString("dace19089043791b92c4421453e314274a6abcda")
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
	expected, err = CommitIDFromString("6060b31388226cc5e3f14166dbe061f68ff180f2")
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
	cid, err = Commit(c, CommitOptions{}, "Changed foo to bar\n", nil)
	if err != nil {
		t.Fatal("Commit with detached head error:", err)
	}
	if cid != expected {
		t.Errorf("Unexpected hash for second commit while in detached head mode. got %v want %v", cid, expected)
	}
	content, err := ioutil.ReadFile(c.GitDir.File("HEAD").String())
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != cid.String() {
		t.Errorf("Commit lied about updating the HEAD reference")
	}
}

// TestUnmergedCommit tests that Commit() produces an
// error when there are merge entries in the index.
func TestUnmergedCommit(t *testing.T) {
	gitdir, err := ioutil.TempDir("", "gitcommitunmerged")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(gitdir)

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

// TestSimpleCommits tests that an initial commit works,
// and that a second commit using it as a parent works.
// It also tests that the second commit can be done while
// in a detached head mode.
func TestCommitAmend(t *testing.T) {
	dir, err := ioutil.TempDir("", "gitcommitamend")
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
	expected, err := CommitIDFromString("dace19089043791b92c4421453e314274a6abcda")
	if err != nil {
		t.Fatal(err)
	}

	if initialCmt != expected {
		t.Errorf("Unexpected hash for expected commit. got %v want %v", initialCmt, expected)
	}

	// Pretend time passed and the author changed. The author date shouldn't
	// be honoured (without --reset-author), it should come from the initial
	// commit. The committer date should be honoured.
	if err := os.Setenv("GIT_AUTHOR_NAME", "Joan Smith"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("GIT_AUTHOR_EMAIL", "tester@example.com"); err != nil {
		t.Fatal(err)
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
	cid, err := Commit(c, CommitOptions{Amend: true}, "Changed foo to bar", nil)
	if err != nil {
		t.Fatal(err)
	}
	expected, err = CommitIDFromString("ef1985dcfdec1e1b3225d6536e1662d28adecf2f")
	if err != nil {
		t.Fatal(err)
	}
	if cid != expected {
		t.Errorf("Unexpected hash for amended commit. got %v want %v", cid, expected)
	}
	cid, err = Commit(c, CommitOptions{Amend: true, ResetAuthor: true}, "Changed foo to bar", nil)
	if err != nil {
		t.Fatal(err)
	}
	expected, err = CommitIDFromString("bde182793e07d43a99565fd102a74437d5762ede")
	if err != nil {
		t.Fatal(err)
	}
	if cid != expected {
		t.Errorf("Unexpected hash for amended commit. got %v want %v", cid, expected)
	}

	if err := ioutil.WriteFile(dir+"/foo.txt", []byte("foo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Add(c, AddOptions{}, []File{"foo.txt"}); err != nil {
		t.Fatal(err)
	}

	cid, err = Commit(c, CommitOptions{}, "Back to the footure", nil)
	if err != nil {
		t.Fatal(err)
	}
	expected, err = CommitIDFromString("6da09b27e260087d6559268fda99369fe6104ced")
	if err != nil {
		t.Fatal(err)
	}
	if cid != expected {
		t.Errorf("Unexpected hash for amended commit. got %v want %v", cid, expected)
	}
	if err := ioutil.WriteFile(dir+"/foo.txt", []byte("baz\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Add(c, AddOptions{}, []File{"foo.txt"}); err != nil {
		t.Fatal(err)
	}

	cid, err = Commit(c, CommitOptions{Amend: true}, "Remove bad pun.", nil)
	if err != nil {
		t.Fatal(err)
	}
	expected, err = CommitIDFromString("eb36fff82a7c1cee518fa0f73548f3d9a873220b")
	if err != nil {
		t.Fatal(err)
	}
	if cid != expected {
		t.Errorf("Unexpected hash for amended commit. got %v want %v", cid, expected)
	}

	if err := os.Setenv("GIT_AUTHOR_NAME", "Foobar"); err != nil {
		t.Fatal(err)
	}
	cid, err = Commit(c, CommitOptions{Amend: true, ResetAuthor: true}, "Remove bad pun.", nil)
	if err != nil {
		t.Fatal(err)
	}
	expected, err = CommitIDFromString("51fb69956f6ea3a8500109a5f1b282ba548cdbf0")
	if err != nil {
		t.Fatal(err)
	}
	if cid != expected {
		t.Errorf("Unexpected hash for amended commit. got %v want %v", cid, expected)
	}

	// FIXME: Add tests for when the tree doesn't get modified and --allow-empty
	// isn't set, also add tests for merge commit amends
}
