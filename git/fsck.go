package git

import (
	"compress/zlib"
	"crypto/sha1"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type FsckOptions struct {
	Unreachable      bool
	NoDangling       bool
	Root             bool
	Tags             bool
	Cache            bool
	NoReflogs        bool
	NoFull           bool
	ConnectivityOnly bool
	Strict           bool
	Verbose          bool
	LostFound        bool
	NameObjects      bool
	NoProgress       bool
}

// Fsck implements the "git fsck" subcommand. It prints any error encountered to
// the stderr argument, and returns an array of said errors.
func Fsck(c *Client, stderr io.Writer, opts FsckOptions, objects []string) (errs []error) {
	addErr := func(err error) {
		fmt.Fprintln(stderr, err)
		errs = append(errs, err)

	}

	if err := verifyHead(c, stderr, opts); err != nil {
		addErr(err)
	}

	if opts.Verbose {
		fmt.Fprintln(stderr, "Checking object directory")
	}

	// HaveObject doesn't do any validation, so we keep track of things
	// we found that are corrupted so we can include error messages if
	// they're used.
	corrupted := make(map[Sha1]struct{})
	objdir := c.GetObjectsDir().String()
	objprefixes, err := ioutil.ReadDir(objdir)
	if err != nil {
		addErr(err)
	} else {
		// FIXME: This should verify the hashes in pack indexes too.
		for _, prefixdir := range objprefixes {
			// We wrap the loop in a closure function so that defers
			// (ie file.Close()) don't need to wait until the entire repo
			// is finished.
			err := func() error {
				// We only want the 2 character prefix directories so that we
				// can check the objects inside of them.
				if !prefixdir.IsDir() {
					return nil
				}
				if len(prefixdir.Name()) != 2 {
					return nil
				}
				objects, err := ioutil.ReadDir(
					filepath.Join(objdir, prefixdir.Name()),
				)
				if err != nil {
					return err
				}
				for _, object := range objects {
					wantsha1 := fmt.Sprintf("%s%s", prefixdir.Name(), object.Name())
					oid, err := Sha1FromString(wantsha1)
					if err != nil {
						return err
					}
					if opts.Verbose {
						fmt.Fprintf(stderr, "Checking %s %s\n", oid.Type(c), wantsha1)
					}
					filename := filepath.Join(objdir, prefixdir.Name(), object.Name())
					f, err := os.Open(filepath.Join(filename))
					if err != nil {
						return err
					}
					defer f.Close()
					zr, err := zlib.NewReader(f)
					if err != nil {
						return err
					}
					h := sha1.New()
					if _, err := io.Copy(h, zr); err != nil {
						return err
					}
					sum := h.Sum(nil)
					sumsha1, err := Sha1FromSlice(sum)
					if err != nil {
						// This should never happen, a sha1 from crypto/sha1
						// should always be convertable to our Sha1 type
						panic(err)
					}
					if sumsha1 != oid {
						corrupted[oid] = struct{}{}
						return fmt.Errorf("hash mismatch for %v (expected %v)", filename, oid)
					}
					switch ty := oid.Type(c); ty {
					case "commit":
						if err := verifyCommit(c, opts, CommitID(oid)); err != nil {
							return fmt.Errorf("error in commit %v: %v", oid, err)
						}
					case "tree":
						if err := verifyTree(c, opts, TreeID(oid)); err != nil {
							return fmt.Errorf("error in tree %v: %v", oid, err)
						}
					case "tag":
						if errs := verifyTag(c, opts, oid); errs != nil {
							for _, err := range errs {
								addErr(err)
							}
							return nil
						}
					case "blob":
						// There's not much to verify for a blob, but it's
						// a known type.
					default:
						return fmt.Errorf("Unknown object type %v", ty)
					}

				}
				return nil
			}()
			if err != nil {
				addErr(err)
			}
		}
	}

	var hc []Commitish
	// Either use RevParse or ShowRef to get a list of all commits that
	// we want to be checking, depending on if anything was passed as
	// an argument.
	if len(objects) != 0 {
		heads, err := RevParse(c, RevParseOptions{}, objects)
		if err != nil {
			addErr(err)
			// We can't do much more if we can't figure out which objects
			// we're supposed to be validating.
			return errs
		}
		for _, head := range heads {
			h, err := head.CommitID(c)
			if err != nil {
				addErr(err)
			}
			hc = append(hc, h)
		}
	} else {
		heads, err := ShowRef(c, ShowRefOptions{}, nil)
		if err != nil {
			addErr(err)
		}
		for _, head := range heads {
			t, err := c.GetObject(head.Value)
			if err != nil {
				addErr(err)
			}
			if t.GetType() == "tag" {
				// This was verified by verifytag
				continue
			}
			h, err := head.CommitID(c)
			if err != nil {
				addErr(fmt.Errorf("not a commit"))
			}
			hc = append(hc, h)
		}

	}

	// Get a list of all reachable objects from the heads.
	reachables, err := RevList(c, RevListOptions{Quiet: true, Objects: true}, nil, hc, nil)
	if err != nil {
		errs = append(errs, err)
		return errs
	}
	for _, obj := range reachables {
		if opts.Verbose {
			fmt.Fprintf(stderr, "Checking %v\n", obj)
		}
		if _, ok := corrupted[obj]; ok {
			addErr(fmt.Errorf("%v corrupt or missing", obj))
			continue
		}
		o, _, err := c.HaveObject(obj)
		if err != nil {
			addErr(err)
			continue
		}
		if !o {
			addErr(fmt.Errorf("%v corrupt or missing", obj))
			continue
		}
	}
	return errs
}

// Verifies the HEAD pointer for fsck.
func verifyHead(c *Client, stderr io.Writer, opts FsckOptions) error {
	if opts.Verbose {
		fmt.Fprintln(stderr, "Checking HEAD link")
	}

	hfile := c.GitDir.File("HEAD")
	if !hfile.Exists() {
		return fmt.Errorf("Missing head link")
	}

	line, err := hfile.ReadFirstLine()
	if err != nil {
		// this shouldn't happen since we already verified it exists
		return err
	}

	sha1, err := Sha1FromString(line)
	if err != nil {
		// we couldn't convert it to a sha1, so it must be a ref
		// pointer and should point to a head (not a tag or a remote)
		if !strings.HasPrefix(line, "ref: refs/heads") {
			return fmt.Errorf("error: HEAD points to something strange")
		}
		return nil
	}

	// We could convert the line to a Sha1, it's a detached head.
	if sha1 == (Sha1{}) {
		return fmt.Errorf("error: HEAD: detached HEAD points at nothing")
	}
	have, _, err := c.HaveObject(sha1)
	if err != nil || !have {
		return fmt.Errorf("error: invalid sha1 pointer %v", sha1)
	}
	return nil
}

func validatePerson(obj GitObject, typ string) error {
	s := getObjectHeader(obj.GetContent(), typ)
	// 0 = whole match
	// 1 = name
	// 2 = email
	// 3 = timestamp
	personRe := regexp.MustCompile(`(.*?)\<(.*?)\>(.*)`)
	pieces := personRe.FindStringSubmatch(s)
	if len(pieces) != 4 {
		// This is mostly just to get the same error messages
		// as git when running the official test suite"
		// "foo asdf> 1234" is reported as bad name
		// "foo 1234" is reported as bad email.
		if strings.Count(s, ">") == 0 {
			return fmt.Errorf("missingEmail: invalid %v line - missing email", typ)
		}
		return fmt.Errorf("badName: invalid %v line - bad name", typ)
	}
	if strings.Count(pieces[1], ">") > 0 {
		return fmt.Errorf("badName: invalid %v line - bad name", typ)
	}
	if !strings.HasPrefix(pieces[3], " ") {
		return fmt.Errorf("missingSpaceBeforeDate: invalid %v line - missing space before date", typ)
	}

	timestampRe := regexp.MustCompile(`^ (\d+) (\+|\-)(\d+)$`)
	timepieces := timestampRe.FindStringSubmatch(pieces[3])
	if len(timepieces) == 0 {
		return fmt.Errorf("invalidateDate: invalid %v line - timestamp is not a valid date", typ)
	}
	// check for overflow of uint64
	bignum, ok := new(big.Int).SetString(timepieces[1], 10)
	if !ok {
		// This shouldn't happen since the regexp validated
		// that it was a string of digits.
		panic("Could not convert integer to bignum")
	}

	// can't use math.Newint because it takes an int64, not a uint64
	maxuint64, ok := new(big.Int).SetString("18446744073709551615", 10)
	if !ok {
		// This shouldn't happen since we're dealing with a const
		panic("Could not convert max uint64 to bignum")
	}
	if bignum.Cmp(maxuint64) > 0 {
		return fmt.Errorf("badDateOverflow: invalid %v line - date causes integer overflow", typ)
	}
	return nil
}

// Verifies a commit for fsck or rev-parse --verify-objects
func verifyCommit(c *Client, opts FsckOptions, cmt CommitID) error {
	obj, err := c.GetCommitObject(cmt)
	if err != nil {
		return err
	}

	if err := validatePerson(obj, "author"); err != nil {
		return err
	}
	if err := validatePerson(obj, "committer"); err != nil {
		return err
	}

	content := obj.GetContent()
	for i, c := range content {
		if c == 0 {
			return fmt.Errorf("nulInHeader: unterminated header: NUL at offset %v", i)
		}
		if c == '\n' && i > 0 && content[i-1] == '\n' {
			// reached the end of the headers.
			break
		}
	}
	return nil
}

// Verifies a tree for fsck or rev-parse --verify-objects
func verifyTree(c *Client, opts FsckOptions, tid TreeID) error {
	paths := make(map[IndexPath]struct{})
	obj, err := c.GetObject(Sha1(tid))
	if err != nil {
		return err
	}
	content := obj.GetContent()
	i := 0
	for i < len(content) {
		name, _, size, err := parseRawTreeLine(i, content)
		if err != nil {
			return err
		}
		if _, ok := paths[name]; ok {
			return fmt.Errorf("duplicateEntries: contains duplicate file entries")
		}
		paths[name] = struct{}{}
		i += size

	}
	return nil
}

func verifyTag(c *Client, opts FsckOptions, tid Sha1) []error {
	var errs []error
	tag, err := c.GetTagObject(tid)
	if err != nil {
		return []error{err}
	}
	objid := tag.GetHeader("object")
	objsha, err := Sha1FromString(objid)
	if err != nil {
		return []error{err}
	}

	_, err = c.GetCommitObject(CommitID(objsha))
	if err != nil {
		// This is really stupid, but t1450.17 expects
		// this one particular error on stdout instead
		// of stderr, so we just print it instead of
		// returning it.
		fmt.Printf(
			`broken link from tag %v
              to commit %v
`, tid, objid,
		)
		errs = append(errs, fmt.Errorf(""))
	}
	if tg := tag.GetHeader("tag"); tg != "" {
		words := strings.Fields(tg)
		if len(words) > 1 {
			// Similar stupidity to t1450.17, t1450.18
			// expects these on stderr, but also expects
			// that these leave an exit status of 0.
			fmt.Fprintf(os.Stderr, "warning in tag %v: badTagName: invalid 'tag' name: wrong name format\n", tid)
		}
	}
	tagger := tag.GetHeader("tagger")
	if tagger == "" {
		fmt.Fprintf(os.Stderr, "warning in tag %v: missingTaggerEntry: invalid format - expected 'tagger' line\n", tid)
	} else if err := validatePerson(tag, "tagger"); err != nil {
		errs = append(errs, fmt.Errorf("error in tag %v: invalid author/committer", tid))
	}

	content := tag.GetContent()
	for i, c := range content {
		if c == 0 {
			errs = append(errs, fmt.Errorf("error in tag %v: nulInHeader: unterminated header: NUL at offset %v", tid, i))
		}
		if c == '\n' && i > 0 && content[i-1] == '\n' {
			// reached the end of the headers.
			break
		}
	}
	return errs
}
