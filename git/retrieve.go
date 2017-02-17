package git

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type PktLine string

func PktLineEncode(line []byte) (PktLine, error) {
	if len(line) > 65535 {
		return "", fmt.Errorf("Line too long to encode in PktLine format")
	}
	return PktLine(fmt.Sprintf("%.4x%s\n", len(line)+5, line)), nil
}

func (p PktLine) String() string {
	return string(p)
}

var InvalidResponse error = errors.New("Invalid response")
var NoNewCommits error = errors.New("No new commits")

type Reference struct {
	Sha1    string
	Refname RefSpec
}

type UpdateReference struct {
	LocalSha1, RemoteSha1 string
	Refname               RefSpec
}
type Uploadpack interface {
	// Retrieves a list of references from the server, using git service
	// "service".
	RetrieveReferences(service string, r io.Reader) (refs []*Reference, capabilities []string, err error)

	// Negotiates a packfile and returns a reader that can
	// read it.
	// As well as a map of refs/ that the server had
	// to the hashes that they reference
	NegotiatePack() ([]*Reference, *os.File, error)

	// Negotiate references that should up uploaded in a sendpack
	NegotiateSendPack() ([]*Reference, error)

	// Sends the PackFile from a Reader and requests that the references in
	// []UpdateReference be updated on the remote server
	SendPack(ref UpdateReference, r io.Reader, size int64) error
}

type SmartHTTPServerRetriever struct {
	Location string
	C        *Client

	username, password string
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

func (s SmartHTTPServerRetriever) RetrieveReferences(service string, r io.Reader) ([]*Reference, []string, error) {
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
				ret.Refname = RefSpec(s[firstSpace:nameEnd])
			}
			if char == '\n' {
				if ret.Refname == "" {
					ret.Refname = RefSpec(s[firstSpace:idx])
				} else {
					capabilities = strings.Split(s[nameEnd:], " ")
					return ret
				}
			}

		}
		return ret
	}

	header := loadLine(r)
	if header != "# service="+service+"\n" {
		return nil, nil, InvalidResponse
	}

	ctrl := loadLine(r)
	if ctrl != "" {
		// Expected a 0000 control line.
		return nil, nil, InvalidResponse
	}
	var references []*Reference
	for line := loadLine(r); line != ""; line = loadLine(r) {
		if line == "" {
			break
		} else {
			references = append(references, parseLine(line))
		}
	}
	return references, capabilities, nil

}
func (s SmartHTTPServerRetriever) parseUploadPackInfoRefs(r io.Reader) ([]*Reference, string, error) {
	var postData string

	var sentData bool = false
	var noDone bool = false
	var responseCapabilities = make([]string, 0)
	references, capabilities, err := s.RetrieveReferences("git-upload-pack", r)
	if err != nil {
		return nil, "", err
	}

	for _, val := range capabilities {
		switch val {
		case "no-done":
			// This seems to require multi_ack_detailed, and I'm
			// not prepared to figure out how to implement that
			// right now.
			//noDone = true
			//fallthrough
		case "ofs-delta":
			fallthrough
		case "no-progress":
			responseCapabilities = append(responseCapabilities, val)
		}

	}
	wantAtLeastOne := false
	for _, ref := range references {
		var line string
		if have, _, _ := s.C.HaveObject(ref.Sha1); have == false {
			if ref.Refname.String() == "HEAD" || ref.Refname.HasPrefix("refs/heads") {
				line = fmt.Sprintf("want %s", ref.Sha1)
				wantAtLeastOne = true
			}
		} else {
			line = fmt.Sprintf("have %s", ref.Sha1)
		}
		if sentData == false {
			if len(responseCapabilities) > 0 {
				line += " " + strings.Join(responseCapabilities, " ")
			}
			sentData = true
		}
		if line != "" {
			postData += fmt.Sprintf("%.4x%s\n", len(line)+5, line)
		}
	}
	if noDone {
		return references, postData + "0000", nil
	}
	if !wantAtLeastOne {
		return references, "", nil
	}
	return references, postData + "00000009done\n", nil
}

func readLine(prompt string) string {
	getInput := bufio.NewReader(os.Stdin)
	var val string
	var err error
	for {
		fmt.Fprintf(os.Stderr, prompt)
		val, err = getInput.ReadString('\n')
		if err != nil {
			return ""
		}

		val = strings.TrimSpace(val)
		if val != "" {
			return val
		}
	}
}

// Retrieves a list of references from the server using service and expecting Content-Type
// of expectedmime
func (s *SmartHTTPServerRetriever) getRefs(service, expectedmime string) (io.ReadCloser, error) {
	// Try directly accessing the server's URL.
	s.Location = strings.TrimSuffix(s.Location, "/")
	req, err := http.NewRequest("GET", s.Location+"/info/refs?service="+service, nil)
	if err != nil {
		return nil, err
	}

	if s.username != "" || s.password != "" {
		println("Setting password ", s.username, s.password)
		req.SetBasicAuth(s.username, s.password)
	}
	resp, err := http.DefaultClient.Do(req)
	if resp.Header.Get("Content-Type") != expectedmime {
		// If it didn't work, close the body and try again at "url.git"
		if err != nil {
			resp.Body.Close()
		}
		s.Location = s.Location + ".git"
		req, err = http.NewRequest("GET", s.Location+"/info/refs?service="+service, nil)
		if s.username != "" || s.password != "" {
			req.SetBasicAuth(s.username, s.password)
		}
		resp, err = http.DefaultClient.Do(req)
	}
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, errors.New(resp.Status)
	}
	// It worked, so return the Body reader. It's the callers responsibility
	// to close it.
	return resp.Body, nil
}

func (s *SmartHTTPServerRetriever) NegotiateSendPack() ([]*Reference, error) {
	var err error

	s.username = readLine("Username: ")
	s.password = readLine("Password: ")
	r, err := s.getRefs("git-receive-pack", "application/x-git-receive-pack-request")
	if err != nil {
		return nil, err
	}
	defer r.Close()

	refs, cap, err := s.RetrieveReferences("git-receive-pack", r)
	for _, c := range cap {
		println(c)
	}
	if err != nil {
		return nil, err
	}
	return refs, nil

}
func (s SmartHTTPServerRetriever) NegotiatePack() ([]*Reference, *os.File, error) {
	r, err := s.getRefs("git-upload-pack", "application/x-git-upload-pack-advertisement")
	if err != nil {
		return nil, nil, err
	}
	defer r.Close()
	refs, toPost, err := s.parseUploadPackInfoRefs(r)
	//refs, toPost, err := s.parseUploadPackInfoRefs(o.TeeReader(r, os.Stderr))
	if err != nil {
		return nil, nil, err
	}
	if toPost == "" {
		fmt.Fprintf(os.Stderr, "Already up to date\n")
		return refs, nil, NoNewCommits
	}
	r2, err := http.Post(s.Location+"/git-upload-pack", "application/x-git-upload-pack-request", strings.NewReader(toPost))
	if err != nil {
		return refs, nil, err
	}
	defer r2.Body.Close()
	response := loadLine(r2.Body)
	if response != "NAK\n" {
		panic(response)
	}

	// Use a tempfile so that the body is a io.ReadSeeker
	f, err := ioutil.TempFile("", "gitpack")
	if err != nil {
		return refs, f, err
	}

	io.Copy(f, r2.Body)

	return refs, f, nil
}

func (s SmartHTTPServerRetriever) SendPack(ref UpdateReference, r io.Reader, size int64) error {
	var toPost string

	line, err := PktLineEncode([]byte(fmt.Sprintf("%s %s %s\000 report-status quiet sideband-64k agent=go-git/0.0.1", ref.RemoteSha1, ref.LocalSha1, ref.Refname.String())))
	if err != nil {
		panic(err)
	}
	toPost += line.String()
	toPost += "0000"

	body := io.MultiReader(strings.NewReader(toPost), r, strings.NewReader("0000"))

	req, err := http.NewRequest("POST", s.Location+"/git-receive-pack", body)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "go-git/0.0.1")

	req.ContentLength = int64(len(toPost)) + size + 4
	req.Header.Set("Content-Type", "application/x-git-receive-pack-request")
	if s.username != "" || s.password != "" {
		req.SetBasicAuth(s.username, s.password)
	}

	resp, err2 := http.DefaultClient.Do(req)
	if err2 != nil {
		panic(err2)
	}
	defer resp.Body.Close()
	if resp.Body == nil {
		panic("Response body is nil :(")
	}
	b, err3 := ioutil.ReadAll(resp.Body)
	fmt.Printf("%s %v %s (%d)\n\n%sdone", resp.Proto, resp.Close, resp.Status, resp.ContentLength, b)
	for key, val := range resp.Header {
		fmt.Printf("%s %s\n", key, val)
	}
	if err3 != nil {
		panic(err3)
	}
	return err3

}
