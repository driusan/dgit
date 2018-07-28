package git

import (
	"fmt"
	"testing"
)

func TestIgnoreMatchStringer(t *testing.T) {
	zero := IgnoreMatch{PathName: File("myfile.txt")}
	if zero.String() != fmt.Sprintf("::\tmyfile.txt") {
		t.Fail()
	}

	nonZero := IgnoreMatch{PathName: File(".././abc/xyz/foo.txt"), IgnorePattern: IgnorePattern{Pattern: "/**/*.txt", LineNum: 5, Source: File("xyz/.gitignore")}}
	if nonZero.String() != fmt.Sprintf("xyz/.gitignore:5:/**/*.txt\t.././abc/xyz/foo.txt") {
		t.Fail()
	}
}

func TestIgnorePatternMatches(t *testing.T) {
	pattern1 := IgnorePattern{Pattern: "*.txt", Scope: File("")}
	if !pattern1.Matches("foo/bar.txt", false) {
		t.Fail()
	}
	if pattern1.Matches("foo/bar.go", false) {
		t.Fail()
	}
	if !pattern1.Matches("bar.txt", false) {
		t.Fail()
	}
	if pattern1.Matches("bar.go", false) {
		t.Fail()
	}

	// Out of scope, should never match
	pattern2 := IgnorePattern{Pattern: "*.txt", Scope: File("other")}
	if pattern2.Matches("foo/bar.txt", false) {
		t.Fail()
	}
	if pattern2.Matches("foo/bar.go", false) {
		t.Fail()
	}
	if pattern2.Matches("bar.txt", false) {
		t.Fail()
	}
	if pattern2.Matches("bar.go", false) {
		t.Fail()
	}

	// Another out of scope, should never match
	pattern3 := IgnorePattern{Pattern: "*.txt", Scope: File("other")}
	if pattern3.Matches("other2/bar.txt", false) {
		t.Fail()
	}
	if pattern3.Matches("other2/bar.go", false) {
		t.Fail()
	}
	if pattern3.Matches("bar.txt", false) {
		t.Fail()
	}
	if pattern3.Matches("bar.go", false) {
		t.Fail()
	}

	// In scope, but with a specific path in the pattern
	pattern4 := IgnorePattern{Pattern: "folder2/test.txt", Scope: File("folder1")}
	if pattern4.Matches("folder1/test.txt", false) {
		t.Fail()
	}
	if !pattern4.Matches("folder1/folder2/test.txt", false) {
		t.Fail()
	}

	// In scope and a directory specific match
	pattern5 := IgnorePattern{Pattern: "folder2/", Scope: File("")}
	if pattern5.Matches("folder2", false) {
		t.Fail()
	}
	if !pattern5.Matches("folder2", true) {
		t.Fail()
	}
	if !pattern5.Matches("folder2/blarg.txt", false) {
		t.Fail()
	}
	if !pattern5.Matches("folder2/blarg.txt", true) {
		t.Fail()
	}

	// In scope with a variable number of folders in the pattern
	pattern6 := IgnorePattern{Pattern: "foo/**/bar.txt", Scope: File("")}
	if !pattern6.Matches("foo/bar.txt", false) {
		t.Fail()
	}
	if !pattern6.Matches("foo/folder1/bar.txt", false) {
		t.Fail()
	}
	if !pattern6.Matches("foo/folder1/folder2/bar.txt", false) {
		t.Fail()
	}

	// Variable number of folders and a regular wildcard
	pattern7 := IgnorePattern{Pattern: "foo/**/*.txt", Scope: File("")}
	if !pattern7.Matches("foo/test.txt", false) {
		t.Fail()
	}
	if !pattern7.Matches("foo/bar/t.txt", false) {
		t.Fail()
	}
}

func TestPatternSort(t *testing.T) {
	patterns := []IgnorePattern{IgnorePattern{Pattern: "abc", Scope: File("")}, IgnorePattern{Pattern: "xyz", Scope: File("folder1")}}
	sortPatterns(&patterns)
	if patterns[0].Scope != "folder1" {
		t.Fail()
	}
}
