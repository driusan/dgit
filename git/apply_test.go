package git

import (
	"io/ioutil"
	"os"
	"testing"
)

// TestApply tests that the basic usage of the "Apply" command works
// as expected and is atomic for the unified diff format.
func TestUnidiffApply(t *testing.T) {
	dir, err := ioutil.TempDir("", "gitapply")
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

	patch, err := ioutil.TempFile("", "applytestpatch")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(patch.Name())
	if err := ioutil.WriteFile(patch.Name(), []byte(
		`diff --git a/foo.txt b/foo.txt
--- a/foo.txt
+++ b/foo.txt
@@ -1 +1 @@
-foo
+bar
`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := Apply(c, ApplyOptions{}, []File{File(patch.Name())}); err != nil {
		t.Fatalf("Error with basic git apply: %v", err)
	}

	file, err := ioutil.ReadFile("foo.txt")
	if err != nil {
		t.Fatal(err)
	}

	if got := string(file); got != "bar\n" {
		t.Fatalf("Unexpected value of foo.txt after simple patch: got %v want %v", got, "bar\n")
	}

	// Make it an invalid patch
	if err := ioutil.WriteFile(patch.Name(), []byte(
		`diff --git a/foo.txt b/foo.txt
--- a/foo.txt
+++ b/foo.txt
@@ -1 +1 @@
-foo
+barr
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Apply(c, ApplyOptions{}, []File{File(patch.Name())}); err == nil {
		t.Fatal("Expected error with invalid patch, got none.")
	}

	file, err = ioutil.ReadFile("foo.txt")
	if err != nil {
		t.Fatal(err)
	}

	if got := string(file); got != "bar\n" {
		t.Fatalf("Unexpected value of foo.txt after invalid patch: got %v want %v", got, "bar\n")
	}

	// Now ensure that the changes are atomic. If they're not, bar.txt will
	// be modified but foo.txt will fail.
	if err := ioutil.WriteFile(dir+"/foo.txt", []byte("foo\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := ioutil.WriteFile(dir+"/bar.txt", []byte("bar\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(patch.Name(), []byte(
		`diff --git a/bar.txt b/bar.txt
--- a/bar.txt
+++ b/bar.txt
@@ -1 +1 @@
-bar
+qux

diff --git a/foo.txt b/foo.txt
--- a/foo.txt
+++ b/foo.txt
@@ -1 +1 @@
-foob
+barr
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Apply(c, ApplyOptions{}, []File{File(patch.Name())}); err == nil {
		t.Fatal("Expected error with invalid patch, got none.")
	}

	file, err = ioutil.ReadFile("foo.txt")
	if err != nil {
		t.Fatal(err)
	}

	if got := string(file); got != "foo\n" {
		t.Fatalf("Unexpected value of foo.txt after invalid patch: got %v want %v", got, "foo\n")
	}
	file, err = ioutil.ReadFile("bar.txt")
	if err != nil {
		t.Fatal(err)
	}

	if got := string(file); got != "bar\n" {
		t.Fatalf("Unexpected value of bar.txt after invalid patch: got %v want %v", got, "bar\n")
	}

	// Now make the work directory such that the patch should apply
	if err := ioutil.WriteFile(dir+"/foo.txt", []byte("foob\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Apply(c, ApplyOptions{}, []File{File(patch.Name())}); err != nil {
		t.Fatalf("Unexpected error with multi-file patch %v", err)
	}

	file, err = ioutil.ReadFile("foo.txt")
	if err != nil {
		t.Fatal(err)
	}

	if got := string(file); got != "barr\n" {
		t.Fatalf("Unexpected value of foo.txt after invalid patch: got %v want %v", got, "barr\n")
	}

	file, err = ioutil.ReadFile("bar.txt")
	if err != nil {
		t.Fatal(err)
	}

	if got := string(file); got != "qux\n" {
		t.Fatalf("Unexpected value of bar.txt after invalid patch: got %v want %v", got, "qux\n")
	}

	// Test that Reverse works as expected
	if err := Apply(c, ApplyOptions{Reverse: true}, []File{File(patch.Name())}); err != nil {
		t.Fatalf("Unexpected error with reverse patch: %v", err)
	}

	file, err = ioutil.ReadFile("foo.txt")
	if err != nil {
		t.Fatal(err)
	}

	if got := string(file); got != "foob\n" {
		t.Fatalf("Unexpected value of foo.txt after invalid patch: got %v want %v", got, "foob\n")
	}

	file, err = ioutil.ReadFile("bar.txt")
	if err != nil {
		t.Fatal(err)
	}

	if got := string(file); got != "bar\n" {
		t.Fatalf("Unexpected value of bar.txt after invalid patch: got %v want %v", got, "bar\n")
	}

	// Ensure that a subdirectory works as expected.
	if err := os.Mkdir("qux", 0755); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(dir+"/qux/qux.txt", []byte("qux\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(patch.Name(), []byte(
		`diff --git a/foo.txt b/foo.txt
--- a/foo.txt
+++ b/foo.txt
@@ -1 +1 @@
-foob
+barr

diff --git a/qux/qux.txt b/qux/qux.txt
--- a/qux/qux.txt
+++ b/qux/qux.txt
@@ -1 +1 @@
-qux
+quux
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Apply(c, ApplyOptions{}, []File{File(patch.Name())}); err != nil {
		t.Fatalf("Unexpected error with file in subdirectory: %v", err)
	}

	file, err = ioutil.ReadFile("foo.txt")
	if err != nil {
		t.Fatal(err)
	}

	if got := string(file); got != "barr\n" {
		t.Fatalf("Unexpected value of foo.txt after invalid patch: got %v want %v", got, "barr\n")
	}

	file, err = ioutil.ReadFile("bar.txt")
	if err != nil {
		t.Fatal(err)
	}

	if got := string(file); got != "bar\n" {
		t.Fatalf("Unexpected value of bar.txt after invalid patch: got %v want %v", got, "bar\n")
	}
	file, err = ioutil.ReadFile("qux/qux.txt")
	if err != nil {
		t.Fatal(err)
	}

	if got := string(file); got != "quux\n" {
		t.Fatalf("Unexpected value of qux/qux.txt after invalid patch: got %v want %v", got, "quux\n")
	}

	// Stage "foo\n" into the index.
	if err := ioutil.WriteFile(dir+"/foo.txt", []byte("foo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Add(c, AddOptions{}, []File{"foo.txt"}); err != nil {
		t.Fatal(err)
	}

	// Put something else on the filesystem
	if err := ioutil.WriteFile(dir+"/foo.txt", []byte("bar\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(patch.Name(), []byte(
		`diff --git a/foo.txt b/foo.txt
--- a/foo.txt
+++ b/foo.txt
@@ -1 +1 @@
-foo
+bara
`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := Apply(c, ApplyOptions{Cached: true}, []File{File(patch.Name())}); err != nil {
		t.Fatalf("Error while applying to index: %v", err)
	}
	file, err = ioutil.ReadFile("foo.txt")
	if err != nil {
		t.Fatal(err)
	}

	if got := string(file); got != "bar\n" {
		t.Fatalf("--cached affected filesystem with foo.txt: got %v want %v", got, "bar\n")
	}

	idx, err := LsFiles(c, LsFilesOptions{Cached: true}, []File{"foo.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx) != 1 || idx[0].PathName != "foo.txt" {
		t.Fatal("LsFiles did not return foo.txt")
	}
	if want := hashString("bara\n"); idx[0].Sha1 != want {
		t.Errorf("Did not apply --cached patch correctly. Got %v want %v", idx[0].Sha1, want)
	}
}
