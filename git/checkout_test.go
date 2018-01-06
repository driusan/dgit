package git

import (
	"io/ioutil"
	"os"
	"testing"
)

// TestCheckoutBranch tests that variations of "git checkout branchname" works
// as expected.
func TestCheckoutBranch(t *testing.T) {
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
	initialCmt, err := Commit(c, CommitOptions{}, "Initial commit", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a branch and check it out.
	if err := c.CreateBranch("notmaster", initialCmt); err != nil {
		t.Fatal(err)
	}
	if err := Checkout(c, CheckoutOptions{}, "notmaster", nil); err != nil {
		t.Fatal(err)
	}

	if err := ioutil.WriteFile(dir+"/foo.txt", []byte("bar\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Add(c, AddOptions{}, []File{"foo.txt"}); err != nil {
		t.Fatal(err)
	}
	// commit to the branch so that our branches are in different states.
	notmasterCmt, err := Commit(c, CommitOptions{}, "notmaster", nil)
	if err != nil {
		t.Fatal(err)
	}

	mastercrev, err := RevParseCommitish(c, &RevParseOptions{}, "master")
	if err != nil {
		t.Fatal(err)
	}
	masterc, err := mastercrev.CommitID(c)

	notmasterrev, err := RevParseCommitish(c, &RevParseOptions{}, "notmaster")
	if err != nil {
		t.Fatal(err)
	}
	notmasterc, err := notmasterrev.CommitID(c)
	if err != nil {
		t.Fatal(err)
	}

	if masterc != initialCmt || notmasterc != notmasterCmt {
		t.Fatal("RevParse did not return the correct branch commit.")
	}

	head, err := SymbolicRefGet(c, SymbolicRefOptions{}, "HEAD")
	if err != nil {
		t.Fatal("Could not get HEAD reference")
	}
	if head != RefSpec("refs/heads/notmaster") {
		t.Errorf("Checkout branch variation did not change head reference. Got: %v", head)
	}

	// Go back to the master branch with a clean working tree
	if err := Checkout(c, CheckoutOptions{}, "master", nil); err != nil {
		t.Fatal(err)
	}
	head, err = SymbolicRefGet(c, SymbolicRefOptions{}, "HEAD")
	if err != nil {
		t.Fatal("Could not get HEAD reference")
	}
	if head != RefSpec("refs/heads/master") {
		t.Errorf("Checkout branch variation did not change head reference. Got: %v", head)
	}

	foo, err := ioutil.ReadFile("foo.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(foo) != "foo\n" {
		t.Error("Incorrect content in foo. Expected foo\n, got %v", string(foo))
	}

	// Go back to notmaster with a dirty working tree. This should fail
	if err := ioutil.WriteFile(dir+"/foo.txt", []byte("baz\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Checkout(c, CheckoutOptions{}, "notmaster", nil); err == nil {
		t.Error("Expected failed checkout due to unmerged changes, got no error")
	}
	foo, err = ioutil.ReadFile("foo.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(foo) != "baz\n" {
		t.Errorf("File was modified by unfinished checkout.")
	}
	if err := Checkout(c, CheckoutOptions{Force: true}, "notmaster", nil); err != nil {
		t.Errorf("Could not force checkout: %v", err)
	}
	foo, err = ioutil.ReadFile("foo.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(foo) != "bar\n" {
		t.Errorf("File was not modified in forced checkout. Got %v want %v", string(foo), "bar")
	}
	if err := Checkout(c, CheckoutOptions{Detach: true}, "master", nil); err != nil {
		t.Errorf("Error detaching checkout: %v", err)
	}
	foo, err = ioutil.ReadFile("foo.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(foo) != "foo\n" {
		t.Errorf("File was not modified in detached checkout. Got %v want %v", string(foo), "foo")
	}

	headfile, err := ioutil.ReadFile(c.GitDir.String() + "/HEAD")
	if err != nil {
		t.Fatal(err)
	}

	if expected := masterc.String(); string(headfile) != expected {
		t.Errorf("Head was not correctly modified by checkout --detach. Got %s want %s", string(headfile), expected)
	}

	// Since we're in a detached head state, test that we can get back
	// to a normal state by checkouting master
	if err := Checkout(c, CheckoutOptions{}, "master", nil); err != nil {
		t.Fatal(err)
	}
	head, err = SymbolicRefGet(c, SymbolicRefOptions{}, "HEAD")
	if err != nil {
		t.Fatal("Could not get HEAD reference")
	}
	if head != RefSpec("refs/heads/master") {
		t.Errorf("Checkout branch variation did not change head from detached head mode. Got: %v", head)
	}
}
