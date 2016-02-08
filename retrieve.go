package main

import (
	"errors"
	"fmt"
	libgit "github.com/driusan/git"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

var InvalidResponse error = errors.New("Invalid response")

type uploadpack interface {
	RefDiscovery(w io.Writer) error
	ReceivePack() error
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
	n, err = r.Read(line)
	if uint64(n) != val-4 || err != nil {
		panic(fmt.Sprintf("Unexpected line size: %d not %d: %s", n, val, line))
	}
	return string(line)

}

func (s smartHTTPServerRetriever) parseUploadPackInfoRefs(r io.Reader) (string, error) {
	var capabilities []string
	type reference struct {
		id      string
		refname string
	}
	var parseLine = func(s string) *reference {
		var ret *reference
		var firstSpace int
		var nameEnd int
		for idx, char := range s {
			if char == ' ' && ret == nil {
				ret = &reference{
					id: s[0:idx],
				}
				firstSpace = idx
			}
			if char == '\000' {
				nameEnd = idx + 1
				ret.refname = s[firstSpace:nameEnd]
			}
			if char == '\n' {
				if ret.refname == "" {
					ret.refname = s[firstSpace:idx]
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
		return "", InvalidResponse
	}

	ctrl := loadLine(r)
	if ctrl != "" {
		// Expected a 0000 control line.
		return "", InvalidResponse
	}
	var references []*reference
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
		line = fmt.Sprintf("want %s", ref.id)
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
		return postData + "0000", nil
	}
	return postData + "00000009done\n", nil
}
func (s smartHTTPServerRetriever) RefDiscovery(w io.Writer) error {
	fmt.Printf("Discovering refs")
	resp, err := http.Get(s.location + "/info/refs?service=git-upload-pack")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	fmt.Printf("Parsing refs")
	toPost, err := s.parseUploadPackInfoRefs(io.TeeReader(resp.Body, os.Stderr))
	if err != nil {
		fmt.Printf("Grah %s", err)
		return err
	}
	fmt.Printf("Grah")
	if toPost == "" {
		fmt.Fprintf(os.Stderr, "Already up to date\n")
		return nil
	}
	fmt.Printf("Asking for pack")
	resp2, err := http.Post(s.location+"/git-upload-pack", "application/x-git-upload-pack-request", strings.NewReader(toPost))
	if err != nil {
		fmt.Printf("%s", err)
		return err
	}
	defer resp2.Body.Close()
	fmt.Printf("Loading line")
	response := loadLine(resp2.Body)
	if response != "NAK\n" {
		panic(response)
	}

	fmt.Printf("Getting pack file")
	_, err = io.Copy(w, resp2.Body)
	return err
}

func (s smartHTTPServerRetriever) ReceivePack() error {
	return nil

}
