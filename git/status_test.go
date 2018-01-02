package git

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
)

type expectedStatus struct {
	Long, Short, ShortBranch, PorcelainV2 string
}

func compareStatus(c *Client, opts StatusOptions, expected expectedStatus) error {
	opts.Short = false
	opts.Branch = false
	opts.Porcelain = 0
	opts.Long = true
	s, err := Status(c, opts, nil)
	if err != nil {
		return fmt.Errorf("Long status: %v", err)
	}
	if expected.Long != s {
		return fmt.Errorf("Long status: got `%v` want `%v`", s, expected.Long)
	}
	opts.Long = false
	opts.Short = true
	s, err = Status(c, opts, nil)
	if err != nil {
		return fmt.Errorf("Short status: %v", err)
	}
	if expected.Short != s {
		return fmt.Errorf("Short status: got `%v` want `%v`", s, expected.Short)
	}
	opts.Branch = true
	s, err = Status(c, opts, nil)
	if err != nil {
		return fmt.Errorf("Short branch status: `%v`", err)
	}
	if expected.ShortBranch != s {
		return fmt.Errorf("Short branch status: got `%v` want `%v`", s, expected.ShortBranch)
	}
	return nil
	/*
		// Porcelain=2 isn't supported yet, so this is commented out
		opts.Porcelain = 2
		opts.Short = false
		opts.Long = false
		s, err = Status(c, opts, nil)
		if err != nil {
			return fmt.Errorf("Porcelain v2 status: %v", err)
		}
		if expected.ShortBranch != s {
			return fmt.Errorf("Porcelain v2 status: got %v want %v", s, expected.PorcelainV2)
		}
	*/
	return nil
}

// TestSimpleCommits tests that an initial commit works,
// and that a second commit using it as a parent works.
// It also tests that the second commit can be done while
// in a detached head mode.
func TestStatus(t *testing.T) {
	dir, err := ioutil.TempDir("", "gitstatus")
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

	if err := compareStatus(c, StatusOptions{}, expectedStatus{
		Long: `On branch master

No commits yet

nothing to commit (create/copy files and use "git add" to track)
`,
		Short:       "",
		ShortBranch: "## No commits yet on master\n",
		PorcelainV2: `# branch.oid (initial)
# branch.head master
`,
	}); err != nil {
		t.Error(err)
	}

	if ioutil.WriteFile("foo.txt", []byte("foo\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := compareStatus(c, StatusOptions{
		UntrackedMode: StatusUntrackedNormal}, expectedStatus{
		Long: `On branch master

No commits yet

Untracked files:
  (use "git add <file>..." to include in what will be committed)

	foo.txt

nothing added to commit but untracked files present (use "git add" to track)
`,
		Short: "?? foo.txt\n",
		ShortBranch: `## No commits yet on master
?? foo.txt
`,
		PorcelainV2: `# branch.oid (initial)
# branch.head master
? foo.txt
`,
	}); err != nil {
		t.Error(err)
	}

	if _, err := Add(c, AddOptions{}, []File{"foo.txt"}); err != nil {
		t.Fatal(err)
	}

	if err := compareStatus(c, StatusOptions{
		UntrackedMode: StatusUntrackedNormal}, expectedStatus{
		Long: `On branch master

No commits yet

Changes to be committed:
  (use "git rm --cached <file>..." to unstage)

	new file:	foo.txt

`,
		Short: "A  foo.txt\n",
		ShortBranch: `## No commits yet on master
A  foo.txt
`,
		PorcelainV2: `# branch.oid (initial)
# branch.head master
1 A. N... 000000 100644 100644 00000000000000000000000000000000000000 257cc5642cb1a054f08cc83f2d943e56fd3ebe99 foo.txt
`,
	}); err != nil {
		t.Error(err)
	}

	if ioutil.WriteFile("foo.txt", []byte("bar\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := compareStatus(c, StatusOptions{
		UntrackedMode: StatusUntrackedNormal}, expectedStatus{
		Long: `On branch master

No commits yet

Changes to be committed:
  (use "git rm --cached <file>..." to unstage)

	new file:	foo.txt

Changes not staged for commit:
  (use "git add <file>..." to update what will be committed)
  (use "git checkout -- <file>..." to discard changes in working directory)

	modified:	foo.txt

`,
		Short: "AM foo.txt\n",
		ShortBranch: `## No commits yet on master
AM foo.txt
`,
		PorcelainV2: `# branch.oid (initial)
# branch.head master
1 AM N... 000000 100644 100644 00000000000000000000000000000000000000 257cc5642cb1a054f08cc83f2d943e56fd3ebe99 foo.txt
`,
	}); err != nil {
		t.Error(err)
	}

	initial, err := Commit(c, CommitOptions{}, "Status test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := compareStatus(c, StatusOptions{
		UntrackedMode: StatusUntrackedNormal}, expectedStatus{
		Long: `On branch master
Changes not staged for commit:
  (use "git add <file>..." to update what will be committed)
  (use "git checkout -- <file>..." to discard changes in working directory)

	modified:	foo.txt

no changes added to commit (use "git add" and/or "git commit -a")
`,
		Short: " M foo.txt\n",
		ShortBranch: `## master
 M foo.txt
`,
		PorcelainV2: `# branch.oid (initial)
# branch.head master
1 .M N... 000000 100644 100644 257cc5642cb1a054f08cc83f2d943e56fd3ebe99 257cc5642cb1a054f08cc83f2d943e56fd3ebe99 foo.txt
`,
	}); err != nil {
		t.Error(err)
	}

	if _, err := Add(c, AddOptions{}, []File{"foo.txt"}); err != nil {
		t.Fatal(err)
	}
	c2, err := Commit(c, CommitOptions{}, "Status test2", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := compareStatus(c, StatusOptions{
		UntrackedMode: StatusUntrackedNormal}, expectedStatus{
		Long: `On branch master
nothing to commit, working tree clean
`,
		Short: "",
		ShortBranch: `## master
`,
		// FIXME: Need to make the commit ID deterministic
		// in order for this to work.
		PorcelainV2: `# branch.oid deadbeefdeadbeefdeadbeefetc
# branch.head master
`,
	}); err != nil {
		t.Error(err)
	}

	if err := CheckoutCommit(c, CheckoutOptions{}, initial); err != nil {
		t.Fatal(err)
	}

	var cid string
	if comm, err := initial.CommitID(c); err != nil && err != DetachedHead {
		t.Fatal(err)
	} else {
		cid = comm.String()
	}

	if err := compareStatus(c, StatusOptions{
		UntrackedMode: StatusUntrackedNormal}, expectedStatus{
		Long: `HEAD detached at ` + cid + `
nothing to commit, working tree clean
`,
		Short: "",
		ShortBranch: `## HEAD (no branch)
`,
		PorcelainV2: `# branch.oid ` + cid + `
# branch.head (detached)
`,
	}); err != nil {
		t.Error(err)
	}

	// Make a third commit so that we can use readtree to produce a conflict.
	if ioutil.WriteFile("foo.txt", []byte("baz\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Add(c, AddOptions{}, []File{"foo.txt"}); err != nil {
		t.Fatal(err)
	}
	c3, err := Commit(c, CommitOptions{}, "Status test3", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReadTreeMerge(c, ReadTreeOptions{}, initial, c2, c3); err != nil {
		t.Fatal(err)
	}
	cid = c3.String()
	if err := compareStatus(c, StatusOptions{
		UntrackedMode: StatusUntrackedNormal}, expectedStatus{
		Long: `HEAD detached at ` + cid + `
Unmerged paths:
  (use "git reset HEAD <file>..." to unstage)
  (use "git add <file>..." to mark resolution)

	both modified:	foo.txt

no changes added to commit (use "git add" and/or "git commit -a")
`,
		Short: "UU foo.txt\n",
		ShortBranch: `## HEAD (no branch)
UU foo.txt
`,
		PorcelainV2: ``, // FIXME: Check what this should be.
	}); err != nil {
		t.Error(err)
	}

	// Finally, put some things in subdirectories and have a mixed status
	// to make sure things work when we're in a subdirectory and to try
	// the different untracked modes.
	if ioutil.WriteFile("bar.txt", []byte("bar\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Add(c, AddOptions{}, []File{"bar.txt"}); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile("bar.txt", []byte("baz\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir("baz", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir("baz2", 0755); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile("baz/bar.txt", []byte("bar\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile("baz/foo.txt", []byte("bar\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile("baz2/bar.txt", []byte("bar\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile("baz2/foo.txt", []byte("bar\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Add(c, AddOptions{}, []File{"baz/foo.txt"}); err != nil {
		t.Fatal(err)
	}
	if err := compareStatus(c, StatusOptions{
		UntrackedMode: StatusUntrackedNo}, expectedStatus{
		Long: `HEAD detached at ` + cid + `
Changes to be committed:
  (use "git reset HEAD <file>..." to unstage)

	new file:	bar.txt
	new file:	baz/foo.txt

Unmerged paths:
  (use "git reset HEAD <file>..." to unstage)
  (use "git add <file>..." to mark resolution)

	both modified:	foo.txt

Changes not staged for commit:
  (use "git add <file>..." to update what will be committed)
  (use "git checkout -- <file>..." to discard changes in working directory)

	modified:	bar.txt

Untracked files not listed (use -u option to show untracked files)
`,
		Short: `AM bar.txt
A  baz/foo.txt
UU foo.txt
`,
		ShortBranch: `## HEAD (no branch)
AM bar.txt
A  baz/foo.txt
UU foo.txt
`,
		PorcelainV2: ``, // FIXME: Check what this should be.
	}); err != nil {
		t.Error(err)
	}
	if os.Chdir("baz2"); err != nil {
		t.Fatal(err)
	}

	// Try it in a subdirectory with -unormal
	if err := compareStatus(c, StatusOptions{
		UntrackedMode: StatusUntrackedNormal}, expectedStatus{
		Long: `HEAD detached at ` + cid + `
Changes to be committed:
  (use "git reset HEAD <file>..." to unstage)

	new file:	../bar.txt
	new file:	../baz/foo.txt

Unmerged paths:
  (use "git reset HEAD <file>..." to unstage)
  (use "git add <file>..." to mark resolution)

	both modified:	../foo.txt

Changes not staged for commit:
  (use "git add <file>..." to update what will be committed)
  (use "git checkout -- <file>..." to discard changes in working directory)

	modified:	../bar.txt

Untracked files:
  (use "git add <file>..." to include in what will be committed)

	../baz/bar.txt
	./

`,
		Short: `AM ../bar.txt
A  ../baz/foo.txt
UU ../foo.txt
?? ../baz/bar.txt
?? ./
`,
		ShortBranch: `## HEAD (no branch)
AM ../bar.txt
A  ../baz/foo.txt
UU ../foo.txt
?? ../baz/bar.txt
?? ./
`,
		PorcelainV2: ``, // FIXME: Check what this should be.
	}); err != nil {
		t.Error(err)
	}

	// Try the same thing with -uall
	if err := compareStatus(c, StatusOptions{
		UntrackedMode: StatusUntrackedAll}, expectedStatus{
		Long: `HEAD detached at ` + cid + `
Changes to be committed:
  (use "git reset HEAD <file>..." to unstage)

	new file:	../bar.txt
	new file:	../baz/foo.txt

Unmerged paths:
  (use "git reset HEAD <file>..." to unstage)
  (use "git add <file>..." to mark resolution)

	both modified:	../foo.txt

Changes not staged for commit:
  (use "git add <file>..." to update what will be committed)
  (use "git checkout -- <file>..." to discard changes in working directory)

	modified:	../bar.txt

Untracked files:
  (use "git add <file>..." to include in what will be committed)

	../baz/bar.txt
	bar.txt
	foo.txt

`,
		Short: `AM ../bar.txt
A  ../baz/foo.txt
UU ../foo.txt
?? ../baz/bar.txt
?? bar.txt
?? foo.txt
`,
		ShortBranch: `## HEAD (no branch)
AM ../bar.txt
A  ../baz/foo.txt
UU ../foo.txt
?? ../baz/bar.txt
?? bar.txt
?? foo.txt
`,
		PorcelainV2: ``, // FIXME: Check what this should be.
	}); err != nil {
		t.Error(err)
	}
}
