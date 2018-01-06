package git

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"
)

// TestGrep tests that the "git grep" subcommand works
func TestGrep(t *testing.T) {
	dir, err := ioutil.TempDir("", "gitgrep")
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

	// foo.txt contains "foo"
	if err := ioutil.WriteFile(dir+"/foo.txt", []byte("foo\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// bar.txt also contains "foo" (on a different line) but isn't tracked
	// at first. It also contains "bar"
	if err := ioutil.WriteFile(dir+"/bar.txt", []byte("bar\nfoo\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := Add(c, AddOptions{}, []File{"foo.txt"}); err != nil {
		t.Fatal(err)
	}

	ob := bytes.Buffer{}
	opts := GrepOptions{
		Stdout: &ob,
	}
	// foo.txt is tracked but doesn't have a commit, grep should find it but
	// not bar when searching for "foo".
	if err := Grep(c, opts, "foo", nil, nil); err != nil {
		t.Error(err)
	}

	if got := ob.String(); got != "foo.txt: foo\n" {
		t.Errorf("Unexepected value: got `%v` want `foo.txt: foo`", got)
	}

	// Try with line numbers
	opts.LineNumbers = true
	ob.Reset()
	if err := Grep(c, opts, "foo", nil, nil); err != nil {
		t.Error(err)
	}

	if got := ob.String(); got != "foo.txt:1: foo\n" {
		t.Errorf("Unexepected value: got `%v` want `foo.txt:1: foo`", got)
	}

	// Add bar and try again.
	if _, err := Add(c, AddOptions{}, []File{"bar.txt"}); err != nil {
		t.Fatal(err)
	}
	opts.LineNumbers = false
	ob.Reset()

	// Should only get 1 line of bar printed.
	if err := Grep(c, opts, "foo", nil, nil); err != nil {
		t.Error(err)
	}

	if got := ob.String(); got != "bar.txt: foo\nfoo.txt: foo\n" {
		t.Errorf("Unexepected value: got `%v` want `bar.txt: foo\nfoo.txt: foo\n`", got)
	}

	opts.LineNumbers = true
	ob.Reset()
	if err := Grep(c, opts, "foo", nil, nil); err != nil {
		t.Error(err)
	}

	if got := ob.String(); got != "bar.txt:2: foo\nfoo.txt:1: foo\n" {
		t.Errorf("Unexepected value: got `%v` want `bar.txt:2: foo\nfoo.txt:1: foo\n`", got)
	}

}
