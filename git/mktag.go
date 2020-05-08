package git

import (
	"fmt"
	"io"
	"io/ioutil"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Implements mktag by reading the input from r in the format
// described in git-mktag(1).
func Mktag(c *Client, r io.Reader) (Sha1, error) {
	// This goes through a ridiculous amount of effort to ensure, not only that we
	// error out in the same conditions as the official git client, but also that we
	// error out with the exact same message (and character position), even when multiple
	// things are wrong or the error message is misleading. The payoff is that we can run
	// the official git test suite against it.
	//
	// This can probably be cleaned up.
	val, err := ioutil.ReadAll(r)
	if err != nil {
		return Sha1{}, err
	}
	if len(val) < 84 {
		// This error matches and length matches the real git command line.
		// (The error message needs to include the word "size wrong" and the prefix
		// "error" to pass the test suite.)
		// I don't know where 84 comes from, I just brute forced the limit from the
		// command line.
		return Sha1{}, fmt.Errorf("error: wanna fool me ? you obviously got the size wrong !")
	}
	strval := string(val)

	if !strings.HasPrefix(strval, "object ") {
		return Sha1{}, fmt.Errorf(`error: char0: does not start with "object "`)
	}
	lines := strings.SplitN(string(strval), "\n", 5)
	_, err = Sha1FromString(strings.TrimPrefix(lines[0], "object "))
	if err != nil {
		return Sha1{}, fmt.Errorf("error: char7: could not get SHA1 hash")
	}
	// It had to be "object sha1" followed by "\ntype ", so the index of
	// "\ntype " had to be char 47, since a sha1 is 40 characters long
	// when written as a hex string.
	if strings.Index(strval, "\ntype ") != 47 {
		return Sha1{}, fmt.Errorf(`error: char47: could not find "\ntype "`)
	}
	if len(lines) < 3 {
		return Sha1{}, fmt.Errorf(`error: char48: could not find next "\n"`)
	}
	if !strings.HasPrefix(lines[2], "tag ") {
		pos := len(lines[0]) + len(lines[1]) + 2
		return Sha1{}, fmt.Errorf(`error: char%d: no "tag " found`, pos)
	}
	typ := strings.TrimPrefix(lines[1], "type ")
	if len(typ) > len("commit") {
		// "commit" is the longest object type name
		return Sha1{}, fmt.Errorf(`error: char53: type too long`)
	}
	switch typ {
	case "commit":
		// FIXME: This should actually verify the object
	case "blob":
	default:
		return Sha1{}, fmt.Errorf(`error: char7: could not verify object.`)
	}
	tagname := strings.TrimPrefix(lines[2], "tag ")
	for i, c := range tagname {
		if unicode.IsControl(c) || c == ' ' {
			// This error is misleading. It did verify it and confirmed that
			// it was invalid, but the test suite checks for this exact message.
			//
			// pos includes 1 for each newline, and 4 for "tag "
			pos := len(lines[0]) + 1 + len(lines[1]) + 1 + 4 + i + 1
			return Sha1{}, fmt.Errorf(`error: char%d: could not verify tag name`, pos)
		}
	}
	if !strings.HasPrefix(lines[3], "tagger ") {
		pos := len(lines[0]) + len(lines[1]) + len(lines[2]) + 3
		return Sha1{}, fmt.Errorf(`error: char%d: could not find "tagger "`, pos)
	}
	tagger := strings.TrimSpace(strings.TrimPrefix(lines[3], "tagger "))
	if tagger[0] == '<' {
		pos := len(lines[0]) + len(lines[1]) + len(lines[2]) + 3 + len("tagger ")
		return Sha1{}, fmt.Errorf(`error: char%d: missing tagger name`, pos)
	}

	taggerRe := regexp.MustCompile(`(.*)\<(.*)\>(.*)`)

	taggerPieces := taggerRe.FindStringSubmatch(tagger)
	// 0 = whole match
	// 1 = author
	// 2 = email
	// 3 = time
	if len(taggerPieces) != 4 {
		pos := len(lines[0]) + len(lines[1]) + len(lines[2]) + 3 + len("tagger ")
		return Sha1{}, fmt.Errorf(`error: char%d: malformed tagger field`, pos)
	}

	if sp := strings.Index(taggerPieces[2], " "); sp >= 0 {
		pos := len(lines[0]) + len(lines[1]) + len(lines[2]) + 3 + len("tagger ")
		return Sha1{}, fmt.Errorf(`error: char%d: malformed tagger field`, pos)
	}

	// It needs to match (\d)+ (\+|\-)\d+, but if the first part doesn't match we need
	// to return a different error than the second to pass the git test suite, so we
	// do this in two parts
	timestampRe := regexp.MustCompile(`^ (\d+) (\+|\-)(\d+)$`)
	timepieces := timestampRe.FindStringSubmatch(taggerPieces[3])
	if len(timepieces) == 0 {
		// This is all really stupid stupid, but we need to ensure that we report
		// the same char position and error message as git to get the tests to pass.

		// If the first part isn't an integer, say it's missing.
		timestampRe2 := regexp.MustCompile(`^ (\d+)`)
		timestampPos := timestampRe2.FindStringSubmatch(taggerPieces[3])
		if len(timestampPos) == 0 {
			pos := len(lines[0]) + len(lines[1]) + len(lines[2]) + 3 + len("tagger ") + len(taggerPieces[1]) + len(taggerPieces[2]) + 3
			return Sha1{}, fmt.Errorf(`error: char%d: missing tag timestamp`, pos)
		}
		if taggerPieces[3][len(timestampPos[1])+1] != ' ' {
			pos := len(lines[0]) + len(lines[1]) + len(lines[2]) + 3 + len("tagger ") + len(taggerPieces[1]) + len(taggerPieces[2]) + 3 + len(timestampPos[1])
			return Sha1{}, fmt.Errorf(`error: char%d: malformed tag timestamp`, pos)
		}
		pos := len(lines[0]) + len(lines[1]) + len(lines[2]) + 3 + len("tagger ") + len(taggerPieces[1]) + len(taggerPieces[2]) + 3 + len(timestampPos[1]) + 1
		return Sha1{}, fmt.Errorf(`error: char%d: malformed tag timezone`, pos)
	}
	tz, err := strconv.Atoi(timepieces[3])
	if err != nil || tz > 1400 {
		// 1400 is the biggest timezone, because after that the +/- wraps around.
		pos := len(lines[0]) + len(lines[1]) + len(lines[2]) + 3 + len("tagger ") + len(taggerPieces[1]) + len(taggerPieces[2]) + 3 + len(timepieces[1]) + len(timepieces[2])
		return Sha1{}, fmt.Errorf(`error: char%d: malformed tag timezone`, pos)
	}
	if lines[4] != "" && lines[4][0] != '\n' {
		// There should have been a blank line
		pos := len(lines[0]) + len(lines[1]) + len(lines[2]) + len(lines[3]) + 4
		return Sha1{}, fmt.Errorf("error: char%d: trailing garbage in tag header", pos)
	}

	// Now that we've done all that work parsing and validating it, throw it all away and just
	// write the object to the objects directory
	return c.WriteObject("tag", val)
}
