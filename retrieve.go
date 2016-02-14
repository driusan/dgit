package main

import (
	"errors"
	"fmt"
	libgit "github.com/driusan/git"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
)

var InvalidResponse error = errors.New("Invalid response")

type Reference struct {
	Sha1    string
	Refname string
}
type uploadpack interface {
	// Negotiates a packfile and returns a reader that can
	// read it.
	// As well as a map of refs/ that the server had
	// to the hashes that they reference
	NegotiatePack() ([]*Reference, *os.File, error)
}

type smartHTTPServerRetriever struct {
	location string
	repo     *libgit.Repository
}

var loadLine = func(r io.Reader) string {
	size := make([]byte, 4)
	n, err := r.Read(size)
	if n != 4 || err != nil {
		return ""
	}
	val, err := strconv.ParseUint(string(size), 16, 64)
	if err != nil {
		return ""
	}
	if val == 0 {
		return ""
	}
	line := make([]byte, val-4)
	n, err = io.ReadFull(r, line)
	if uint64(n) != val-4 || err != nil {
		panic(fmt.Sprintf("Unexpected line size: %d not %d: %s", n, val, line))
	}
	return string(line)

}

func (s smartHTTPServerRetriever) parseUploadPackInfoRefs(r io.Reader) ([]*Reference, string, error) {
	var capabilities []string
	var parseLine = func(s string) *Reference {
		var ret *Reference
		var firstSpace int
		var nameEnd int
		for idx, char := range s {
			if char == ' ' && ret == nil {
				ret = &Reference{
					Sha1: s[0:idx],
				}
				firstSpace = idx
			}
			if char == '\000' {
				nameEnd = idx + 1
				ret.Refname = s[firstSpace:nameEnd]
			}
			if char == '\n' {
				if ret.Refname == "" {
					ret.Refname = s[firstSpace:idx]
				} else {
					capabilities = strings.Split(s[nameEnd:], " ")
					return ret
				}
			}

		}
		return ret
	}

	header := loadLine(r)
	if header != "# service=git-upload-pack\n" {
		return nil, "", InvalidResponse
	}

	ctrl := loadLine(r)
	if ctrl != "" {
		// Expected a 0000 control line.
		return nil, "", InvalidResponse
	}
	var references []*Reference
	for line := loadLine(r); line != ""; line = loadLine(r) {
		if line == "" {
			break
		} else {
			references = append(references, parseLine(line))
		}
	}
	var postData string

	var sentData bool = false
	var noDone bool = false
	var responseCapabilities = make([]string, 0)
	for _, val := range capabilities {
		switch val {
		case "no-done":
			// This seems to require multi_ack_detailed, and I'm
			// not prepared to figure out how to implement that
			// right now.
			//noDone = true
			//fallthrough
		case "ofs-delta":
			//		fallthrough
		case "no-progress":
			responseCapabilities = append(responseCapabilities, val)
		}

	}
	for _, ref := range references {
		var line string
		//if have, _, _ := s.repo.HaveObject(ref.id); have == false {
		line = fmt.Sprintf("want %s", ref.Sha1)
		if sentData == false {
			if len(responseCapabilities) > 0 {
				line += " " + strings.Join(responseCapabilities, " ")
			}
			sentData = true
		}
		postData += fmt.Sprintf("%.4x%s\n", len(line)+5, line)
		//}
	}
	if noDone {
		return references, postData + "0000", nil
	}
	return references, postData + "00000009done\n", nil
}
func (s smartHTTPServerRetriever) NegotiatePack() ([]*Reference, *os.File, error) {

	s.location = strings.TrimSuffix(s.location, "/")
	resp, err := http.Get(s.location + "/info/refs?service=git-upload-pack")
	if resp.Header.Get("Content-Type") != "application/x-git-upload-pack-advertisement" {
		if err != nil {
			resp.Body.Close()
		}
		s.location = s.location + ".git"
		resp, err = http.Get(s.location + "/info/refs?service=git-upload-pack")
	}
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	refs, toPost, err := s.parseUploadPackInfoRefs(io.TeeReader(resp.Body, os.Stderr))
	if err != nil {
		return nil, nil, err
	}
	if toPost == "" {
		fmt.Fprintf(os.Stderr, "Already up to date\n")
		return refs, nil, nil
	}
	resp2, err := http.Post(s.location+"/git-upload-pack", "application/x-git-upload-pack-request", strings.NewReader(toPost))
	if err != nil {
		return refs, nil, err
	}
	defer resp2.Body.Close()
	response := loadLine(resp2.Body)
	if response != "NAK\n" {
		panic(response)
	}

	f, _ := ioutil.TempFile("", "gitpack")
	io.Copy(f, resp2.Body)

	return refs, f, nil
}
