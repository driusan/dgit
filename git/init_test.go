package git

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestInitRepo(t *testing.T) {
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
		t.Error("Unexpected GitDir in client. got %v want %v", c.GitDir, e)
	}
	head, err := c.GitDir.ReadFile("HEAD")
	if err != nil {
		t.Error(err)
	}
	if e := "ref: refs/heads/master\n"; string(head) != e {
		t.Errorf("Unexpected head reference. got %v want %v", string(head), e)
	}
}
