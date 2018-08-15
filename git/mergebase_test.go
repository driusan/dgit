package git

import (
	"io/ioutil"
	"os"
	"testing"
)

// Does the setup for TestMergeBase so that the test can focus on
// the real merge-base test.
// This sets up a tree that looks like
//
//   B
//  /
// A
//  \
//   C
//
// So that we can test both fast-forward and divergent merge-bases
// in a simple case.
// The caller must cleanup tmpdir when done.
func testMergeBaseSimpleSetup(t *testing.T) (c *Client, tmpdir string, A, B, C CommitID) {
	t.Helper()

	dir, err := ioutil.TempDir("", "gitmergebase")
	if err != nil {
		t.Fatal(err)
	}
	tmpdir = dir

	// Init a repo to test an initial commit in.
	c, err = Init(nil, InitOptions{Quiet: true}, dir)
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
	initialCmt, err := Commit(c, CommitOptions{}, "Initial commit", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a branch named "B", and check it out to add a new
	// commit to it.
	if err := c.CreateBranch("B", initialCmt); err != nil {
		t.Fatal(err)
	}
	if err := Checkout(c, CheckoutOptions{}, "B", nil); err != nil {
		t.Fatal(err)
	}

	if err := ioutil.WriteFile(dir+"/foo.txt", []byte("bar\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Add(c, AddOptions{}, []File{"foo.txt"}); err != nil {
		t.Fatal(err)
	}
	// commit to the branch so that our branches are in different states.
	bCmt, err := Commit(c, CommitOptions{}, "Created branch B", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a branch named "C", and based on A, check it out to
	// add a new commit to it.
	if err := c.CreateBranch("C", initialCmt); err != nil {
		t.Fatal(err)
	}
	if err := Checkout(c, CheckoutOptions{}, "C", nil); err != nil {
		t.Fatal(err)
	}

	if err := ioutil.WriteFile(dir+"/foo.txt", []byte("baz\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Add(c, AddOptions{}, []File{"foo.txt"}); err != nil {
		t.Fatal(err)
	}
	// commit to the branch so that our branches are in different states.
	cCmt, err := Commit(c, CommitOptions{}, "Created branch C", nil)
	if err != nil {
		t.Fatal(err)
	}
	return c, dir, initialCmt, bCmt, cCmt
}

// Test that simple merge-base usage works as expected
func TestMergeBaseSimple(t *testing.T) {
	// Test that merge-base of a fast-forward A -> B and
	// B -> A both resolve to A.
	c, dir, A, B, C := testMergeBaseSimpleSetup(t)
	defer os.RemoveAll(dir)

	ab, err := MergeBase(c, MergeBaseOptions{}, []Commitish{A, B})
	if err != nil {
		t.Error(err)
	}
	if ab != A {
		t.Errorf("Unexpected merge base for fast-forward(1): got %v want %v", ab, A)
	}
	ba, err := MergeBase(c, MergeBaseOptions{}, []Commitish{B, A})
	if err != nil {
		t.Error(err)
	}
	if ba != A {
		t.Errorf("Unexpected merge base for fast-forward(2): got %v want %v", ba, A)
	}

	// Test that in the case where they're not direct ancestors, both
	// B C and C B have a merge-base of A.
	bc, err := MergeBase(c, MergeBaseOptions{}, []Commitish{B, C})
	if err != nil {
		t.Error(err)
	}
	if bc != A {
		t.Errorf("Unexpected merge base for tree(1): got %v want %v", bc, A)
	}
	cb, err := MergeBase(c, MergeBaseOptions{}, []Commitish{C, B})
	if err != nil {
		t.Error(err)
	}
	if cb != A {
		t.Errorf("Unexpected merge base for tree(2): got %v want %v", cb, A)
	}
}
