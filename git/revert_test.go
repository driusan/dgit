package git

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestRevert(t *testing.T) {
	dir, err := ioutil.TempDir("", "gitrevert")
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

	// commit a file with "foo", add "bar", add "baz", then revert the "bar".
	// Should end up with foo baz.
	if err := ioutil.WriteFile(dir+"/foo.txt", []byte("foo\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := Add(c, AddOptions{}, []File{"foo.txt"}); err != nil {
		t.Fatalf("Could not add foo.txt %v", err)
	}

	if _, err := Commit(c, CommitOptions{}, "foo", nil); err != nil {
		t.Fatal("Could not commit foo")
	}

	if err := ioutil.WriteFile(dir+"/foo.txt", []byte("foo\nbar\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Add(c, AddOptions{}, []File{"foo.txt"}); err != nil {
		t.Fatalf("Could not add foo.txt %v", err)
	}

	bar, err := Commit(c, CommitOptions{}, "bar", nil)
	if err != nil {
		t.Fatal("Could not commit foo")
	}

	if err := ioutil.WriteFile(dir+"/foo.txt", []byte("foo\nbar\nbaz\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Try  a revert with a dirty tree, should ge an error.
	if err := Revert(c, RevertOptions{}, []Commitish{bar}); err == nil {
		t.Fatal("Was able to revert with a dirty tree.")
	}
	if _, err := Add(c, AddOptions{}, []File{"foo.txt"}); err != nil {
		t.Fatalf("Could not add foo.txt %v", err)
	}

	last, err := Commit(c, CommitOptions{}, "baz", nil)
	if err != nil {
		t.Fatal("Could not commit foo")
	}

	// Revert the last commit, so that we know there won't be conflicts..
	if err := Revert(c, RevertOptions{Edit: false}, []Commitish{last}); err != nil {
		t.Error(err)
	}

	content, err := ioutil.ReadFile("foo.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "foo\nbar\n" {
		t.Errorf("Unexpected content of foo.txt after revert. got %v want %v", string(content), "foo\nbar\n")
	}
}
