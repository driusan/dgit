package git

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestCloneRepo(t *testing.T) {
	dir, err := ioutil.TempDir("", "gitinit")
	if err != nil {
		t.Fatal(err)
	}
	// We just used TempDir to get a file name. Remove it,
	// to ensure that Init creates it.
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	c, err := Init(nil, InitOptions{Quiet: true}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if e := GitDir(dir) + "/.git"; c.GitDir != e {
		t.Errorf("Unexpected GitDir in client. got %v want %v", c.GitDir, e)
	}
	head, err := c.GitDir.ReadFile("HEAD")
	if err != nil {
		t.Error(err)
	}
	if e := "ref: refs/heads/master\n"; string(head) != e {
		t.Errorf("Unexpected head reference. got %v want %v", string(head), e)
	}

	config, err := LoadLocalConfig(c)
	if err != nil {
		t.Error(err)
	}

	repoid := "https://github.com/driusan/dgit.git"

	config.SetConfig("remote.origin.url", repoid)
	config.SetConfig("branch.master.remote", "origin")
	config.SetConfig("branch.master.merge", "refs/heads/master")

	err = config.WriteConfig()
	if err != nil {
		t.Error(err)
	}

	err = FetchRepository(c, FetchOptions{}, "origin")
	if err != nil {
		t.Error(err)
	}

	if err := os.MkdirAll(c.GitDir.File("logs").String(), 0755); err != nil {
		t.Error(err)
	}

	cmtish, err := RevParseCommitish(c, &RevParseOptions{}, "origin/master")
	if err != nil {
		t.Error(err)
	}
	cmt, err := cmtish.CommitID(c)
	if err != nil {
		t.Error(err)
	}

	if err := UpdateRefSpec(
		c,
		UpdateRefOptions{CreateReflog: true, OldValue: CommitID{}},
		RefSpec("refs/heads/master"),
		cmt,
		"clone: "+repoid,
	); err != nil {
		t.Error(err)
	}

	reflog, err := c.GitDir.ReadFile("logs/refs/heads/master")
	if err != nil {
		t.Error(err)
	}

	if err := c.GitDir.WriteFile("logs/HEAD", reflog, 0755); err != nil {
		t.Error(err)
	}

	if err := Reset(c, ResetOptions{Hard: true}, []File{}); err != nil {
		t.Error(err)
	}

}
