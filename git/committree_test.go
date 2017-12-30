package git

import (
	"io/ioutil"
	"os"
	"testing"
)

// Tests that the date parsing used by GIT_COMMITTER_DATE and
// GIT_AUTHOR_DATE works as expected for the formats defined
// in git-commit-tree(1)
func TestParseDate(t *testing.T) {
	tests := []struct {
		EnvString     string
		GitFormat     string
		ExpectedError bool
	}{
		{
			// RFC 2822
			"Fri, 29 Dec 2017 19:19:25 -0500",
			"1514593165 -0500",
			false,
		},
		{
			// ISO 8601 gets converted to GMT since
			// there's no timezone..
			"2017-12-29T19:19:25",
			"1514575165 +0000",
			false,
		},
		{
			// git internal format
			"1514593165 -0500",
			"1514593165 -0500",
			false,
		},
	}

	for i, tc := range tests {
		ptime, err := parseDate(tc.EnvString)
		if tc.ExpectedError {
			if err == nil {
				t.Errorf("Case %d: expected error, got none.", i)
				continue
			}
		} else {
			if err != nil {
				t.Errorf("Case %d: %v", i, err)
				continue
			}
		}

		if gitstr := timeToGitTime(ptime); gitstr != tc.GitFormat {
			t.Errorf("Case %d: got %v want %v", i, gitstr, tc.GitFormat)
		}
	}
}

// Test that commit-tree creates the same commit ids as the official
// git client, for some known commits.
func TestCommitTree(t *testing.T) {
	// Test cases always use the empty tree for an easy, simple
	// value since write-tree has its own separate tests
	tests := []struct {
		AuthorName, AuthorEmail, AuthorDate string
		CommitName, CommitEmail, CommitDate string
		Parents                             []string
		Message                             string
		ExpectedCommitID                    string
	}{
		// An initial commit with no parents or message
		{
			"John Smith", "test@example.com", "Fri, 29 Dec 2017 20:43:22 -0500", // Author
			"John Smith", "test@example.com", "Fri, 29 Dec 2017 20:43:22 -0500", // Committer
			nil,
			"",
			"82066825bb42aa51b8efda3ce0b3a5ded118467d",
		},
		// An initial commit with a message
		{
			"John Smith", "test@example.com", "Fri, 29 Dec 2017 20:43:22 -0500", // Author
			"John Smith", "test@example.com", "Fri, 29 Dec 2017 20:43:22 -0500", // Committer
			nil,
			"I am a test\n",
			"6f22a18883a3a1150faee61e506e16557535067d",
		},
		// A normal commit with the second test case as a parent.
		{
			"John Smith", "test@example.com", "Fri, 29 Dec 2017 20:43:22 -0500", // Author
			"John Smith", "test@example.com", "Fri, 29 Dec 2017 20:43:22 -0500", // Committer
			[]string{"6f22a18883a3a1150faee61e506e16557535067d"},
			"I am a second commit\n",
			"8b140ee15d13a88024414509b2a3ca1b2588fc9b",
		},
		// A merge commit with the first and second commit as parents. This
		// is a weird, but valid tree, and we've already calculated the commits
		// for the parents, we just reuse them for the test..
		{
			"John Smith", "test@example.com", "Fri, 29 Dec 2017 20:43:22 -0500", // Author
			"John Smith", "test@example.com", "Fri, 29 Dec 2017 20:43:22 -0500", // Committer
			[]string{
				"6f22a18883a3a1150faee61e506e16557535067d",
				"8b140ee15d13a88024414509b2a3ca1b2588fc9b",
			},
			"I am a merge commit",
			"551997ac02954d312c6112a1076725f7c43343dc",
		},
		// FIXME: Add merge commit tests
	}

	gitdir, err := ioutil.TempDir("", "gitcommittree")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(gitdir)

	c, err := NewClient(gitdir, "")
	if err != nil {
		t.Fatal(err)
	}

	treeid, err := writeTree(c, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, tc := range tests {
		// Set all appropriate environment variables so that they're
		// known values for testing, and not the user config or
		// time.Now()
		if err := os.Setenv("GIT_AUTHOR_NAME", tc.AuthorName); err != nil {
			t.Fatal(err)
		}
		if err := os.Setenv("GIT_AUTHOR_EMAIL", tc.AuthorEmail); err != nil {
			t.Fatal(err)
		}
		if err := os.Setenv("GIT_AUTHOR_DATE", tc.AuthorDate); err != nil {
			t.Fatal(err)
		}
		if err := os.Setenv("GIT_COMMITTER_NAME", tc.CommitName); err != nil {
			t.Fatal(err)
		}
		if err := os.Setenv("GIT_COMMITTER_EMAIL", tc.CommitEmail); err != nil {
			t.Fatal(err)
		}
		if err := os.Setenv("GIT_COMMITTER_DATE", tc.CommitDate); err != nil {
			t.Fatal(err)
		}

		var parents []CommitID
		for _, p := range tc.Parents {
			psha, err := CommitIDFromString(p)
			if err != nil {
				t.Fatal(err)
			}
			parents = append(parents, psha)
		}
		cid, err := CommitTree(c, CommitTreeOptions{}, TreeID(treeid), parents, tc.Message)
		if err != nil {
			t.Error(err)
		}
		exsha, err := CommitIDFromString(tc.ExpectedCommitID)
		if err != nil {
			t.Fatal(err)
		}
		if exsha != cid {
			t.Errorf("Case %d: got %v want %v", i, cid, exsha)
		}
	}

}
