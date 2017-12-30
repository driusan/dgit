package git

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	//	"strings"
	"regexp"
	"time"
)

type GPGKeyId string
type CommitTreeOptions struct {
	GPGKey GPGKeyId

	NoGPGSign bool
}

func parseDate(str string) (time.Time, error) {
	// RFC 2822
	if t, err := time.Parse("Mon, 02 Jan 2006 15:04:05 -0700", str); err == nil {
		return t, nil
	}
	// ISO 8601
	if t, err := time.Parse("2006-01-02T15:04:05", str); err == nil {
		return t, nil
	}
	// Git Internal format doesn't parse with time.Parse, so we manually parse it..
	re, err := regexp.Compile("([0-9]+) ([+-])([0-9]{4})")
	if err != nil {
		panic(err)
	}
	if pieces := re.FindStringSubmatch(str); len(pieces) == 4 {
		// Create the time seconds since the epoch
		utime, _ := strconv.Atoi(pieces[1])
		t := time.Unix(int64(utime), 0)

		// Take the hour of the timezone and convert it to seconds
		// from UTC.
		// FIXME: This should deal with half hour timezones
		// properly.
		tz, _ := strconv.Atoi(pieces[3][0:2])
		if pieces[2] == "-" {
			tz *= -1
		}
		tz *= 60 * 60
		zone := time.FixedZone(pieces[2]+pieces[3], tz)
		return t.In(zone), nil
	}
	return time.Time{}, fmt.Errorf("Unsupported date format")
}

func CommitTree(c *Client, opts CommitTreeOptions, tree Treeish, parents []CommitID, message string) (CommitID, error) {
	content := bytes.NewBuffer(nil)

	treeid, err := tree.TreeID(c)
	if err != nil {
		return CommitID{}, err
	}
	fmt.Fprintf(content, "tree %s\n", treeid)
	for _, val := range parents {
		fmt.Fprintf(content, "parent %s\n", val)
	}

	var t time.Time
	var author, committer Person
	if date := os.Getenv("GIT_AUTHOR_DATE"); date != "" {
		t, err := parseDate(date)
		if err != nil {
			return CommitID{}, err
		}
		author = c.GetAuthor(&t)
	} else {
		t = time.Now()
		author = c.GetAuthor(&t)
	}
	if date := os.Getenv("GIT_COMMITTER_DATE"); date != "" {
		t, err := parseDate(date)
		if err != nil {
			return CommitID{}, err
		}
		committer = c.GetCommitter(&t)
	} else {
		t = time.Now()
		committer = c.GetCommitter(&t)
	}

	fmt.Fprintf(content, "author %s\n", author)
	fmt.Fprintf(content, "committer %s\n\n", committer)
	fmt.Fprintf(content, "%s", message)
	sha1, err := c.WriteObject("commit", content.Bytes())
	return CommitID(sha1), err
}
