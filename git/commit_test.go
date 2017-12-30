package git

import (
	"io/ioutil"
	"os"
	"testing"
)

// TestSimpleCommits tests that an initial commit works,
// and that a second commit using it as a parent works.
func TestSimpleCommits(t *testing.T) {
	dir, err := ioutil.TempDir("", "gitcommit")
	if err != nil {
		t.Fatal(err)
	}
	//	defer os.RemoveAll(dir)

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
