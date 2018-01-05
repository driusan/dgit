package git

import (
	"io/ioutil"
	"os"
	"testing"
)

// TestResetFiles tests that the "git reset -- pathspec"
// variation of git reset works as expected.
func TestResetFiles(t *testing.T) {
	dir, err := ioutil.TempDir("", "gitreset")
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

	// Create a commit so that there's a HEAD to reset from.
	if err := ioutil.WriteFile(dir+"/foo.txt", []byte("foo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Add(c, AddOptions{}, []File{"foo.txt"}); err != nil {
		t.Fatal(err)
	}

	cmt, err := Commit(c, CommitOptions{}, "Initial commit", nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := ioutil.WriteFile(dir+"/foo.txt", []byte("bar\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := Add(c, AddOptions{}, []File{"foo.txt"}); err != nil {
		t.Fatal(err)
	}

	if err := Reset(c, ResetOptions{}, []File{"foo.txt"}); err != nil {
		t.Error(err)
	}

	idx, err := LsFiles(c, LsFilesOptions{Cached: true}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if sha := hashString("foo\n"); idx[0].Sha1 != sha {
		t.Errorf("Unexpected hash for %v: got %v want %v", idx[0].PathName, idx[0].Sha1, sha)
	}

	if err := ioutil.WriteFile(dir+"/bar.txt", []byte("bar\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := Add(c, AddOptions{}, []File{"bar.txt"}); err != nil {
		t.Fatal(err)
	}

	if err := Reset(c, ResetOptions{}, []File{"bar.txt"}); err != nil {
		t.Error(err)
	}

	idx, err = LsFiles(c, LsFilesOptions{Cached: true}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(idx) != 1 {
		t.Fatal("Did not correctly unstage new file")
	}

	if idx[0].PathName != "foo.txt" || idx[0].Sha1 != hashString("foo\n") {
		t.Error("Unstaged the wrong file.")
	}

	// Get into a detached HEAD state and try again.
	if err := CheckoutCommit(c, CheckoutOptions{Force: true}, cmt); err != nil {
		t.Fatal(err)
	}
	if _, err := SymbolicRefGet(c, SymbolicRefOptions{}, "HEAD"); err != DetachedHead {
		t.Fatal("Did not correctly switch to detached head mode.")
	}
	if err := ioutil.WriteFile(dir+"/foo.txt", []byte("bar\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := Add(c, AddOptions{}, []File{"foo.txt"}); err != nil {
		t.Fatal(err)
	}

	if err := Reset(c, ResetOptions{}, []File{"foo.txt"}); err != nil {
		t.Error(err)
	}

	idx, err = LsFiles(c, LsFilesOptions{Cached: true}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if sha := hashString("foo\n"); idx[0].Sha1 != sha {
		t.Errorf("Unexpected hash for %v: got %v want %v", idx[0].PathName, idx[0].Sha1, sha)
	}
	if _, err := Add(c, AddOptions{}, []File{"bar.txt"}); err != nil {
		t.Fatal(err)
	}

	if err := Reset(c, ResetOptions{}, []File{"bar.txt"}); err != nil {
		t.Error(err)
	}

	idx, err = LsFiles(c, LsFilesOptions{Cached: true}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(idx) != 1 {
		t.Fatal("Did not correctly unstage new file")
	}

	if idx[0].PathName != "foo.txt" || idx[0].Sha1 != hashString("foo\n") {
		t.Error("Unstaged the wrong file.")
	}
}
