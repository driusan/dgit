package git

import (
	"io/ioutil"
	"os"
	"testing"
)

// TestCheckoutIndex does simple tests for basic checkout index usage.
func TestCheckoutIndex(t *testing.T) {
	dir, err := ioutil.TempDir("", "gitcheckoutindex")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Init a repo to test in.
	c, err := Init(nil, InitOptions{Quiet: true}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	// Add 2 files to the index.
	if err := ioutil.WriteFile(dir+"/foo.txt", []byte("foo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(dir+"/bar.txt", []byte("bar\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Add(c, AddOptions{}, []File{"foo.txt", "bar.txt"}); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(dir+"/foo.txt", []byte("WRONG!\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(dir+"/bar.txt", []byte("WRONG!\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Do the test: Checkout one.
	if err := CheckoutIndex(c, CheckoutIndexOptions{Force: true}, []File{"foo.txt"}); err != nil {
		t.Fatal(err)
	}

	// Make sure the content is right.
	file, err := ioutil.ReadFile("foo.txt")
	if err != nil {
		t.Fatal(err)
	}

	if string(file) != "foo\n" {
		t.Errorf("Unexpected content of foo.txt after checkout-index: got %v want %v", string(file), "foo\n")
	}
	// Make sure the content is right.
	file, err = ioutil.ReadFile("bar.txt")
	if err != nil {
		t.Fatal(err)
	}

	if string(file) != "WRONG!\n" {
		t.Errorf("Unexpected content of bar.txt after checkout-index: got %v want %v", string(file), "WRONG!\n")
	}

	// Make sure ls-files says now there's only 1 modified.
	modified, err := LsFiles(c, LsFilesOptions{Modified: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(modified) != 1 {
		t.Errorf("Unexpected number of modified files after checkout-index: got %v want %v", len(modified), 1)
	}
	// Now try checkout-index -a
	// Do the test: Checkout one.
	if err := CheckoutIndex(c, CheckoutIndexOptions{Force: true, All: true}, nil); err != nil {
		t.Fatal(err)
	}

	// Make sure the content is right.
	file, err = ioutil.ReadFile("foo.txt")
	if err != nil {
		t.Fatal(err)
	}

	if string(file) != "foo\n" {
		t.Errorf("Unexpected content of foo.txt after checkout-index: got %v want %v", string(file), "foo\n")
	}
	// Make sure the content is right.
	file, err = ioutil.ReadFile("bar.txt")
	if err != nil {
		t.Fatal(err)
	}

	if string(file) != "bar\n" {
		t.Errorf("Unexpected content of bar.txt after checkout-index: got %v want %v", string(file), "bar\n")
	}

	// Make sure ls-files says now there's only 1 modified.
	modified, err = LsFiles(c, LsFilesOptions{Modified: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(modified) != 0 {
		t.Errorf("Unexpected number of modified files after checkout-index: got %v want %v", len(modified), 0)
	}

}
