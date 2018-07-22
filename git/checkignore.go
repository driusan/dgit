package git

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type CheckIgnoreOptions struct {
}

type IgnoreMatch struct {
	PathName File
	Pattern  string
	Source   File
	LineNum  int
}

func (im IgnoreMatch) String() string {
	lineNum := ""
	if im.LineNum != 0 {
		lineNum = strconv.Itoa(im.LineNum)
	}
	return fmt.Sprintf("%s:%v:%s\t%s", im.Source, lineNum, im.Pattern, im.PathName)
}

func CheckIgnore(c *Client, opts CheckIgnoreOptions, paths []File) ([]IgnoreMatch, error) {
	patternMatches := make([]IgnoreMatch, len(paths))

	for idx, path := range paths {
		log.Printf("Checking if %s is tracked by git\n", path.String())
		entries, err := LsFiles(c, LsFilesOptions{Cached: true, ExcludeStandard: false}, []File{path})
		if err != nil {
			return nil, err
		}

		// As a default, nothing that is tracked by git is ignored
		if len(entries) > 0 {
			log.Printf("Path %v is tracked by git and not ignored.\n", path)
			continue
		}

		abs, err := filepath.Abs(path.String())
		if err != nil {
			return nil, err
		}

		// Let's check that this path is in the git work dir first
		wdpath, err := filepath.Rel(c.WorkDir.String(), abs)
		if err != nil || wdpath == "." {
			return nil, fmt.Errorf("Path %v is not in the git work directory.", path.String())
		}

		dir := filepath.Dir(abs)

		for {
			gitignore := filepath.Join(dir, ".gitignore")
			log.Printf("Checking .gitignore in %s\n", gitignore)
			pattern, lineNumber, err := findPatternInGitIgnore(c, gitignore, wdpath)
			if err != nil {
				return nil, err
			}

			if pattern != "" {
				gitignore, _ = filepath.Rel(c.WorkDir.String(), gitignore)
				patternMatches[idx] = IgnoreMatch{Pattern: pattern, LineNum: lineNumber, Source: File(gitignore), PathName: path}

				break
			}

			if dir == c.WorkDir.String() {
				break
			}
			dir = filepath.Dir(dir)
		}

		// Be sure to assign the pathname in all cases so that clients can match the outputs to their inputs
		patternMatches[idx].PathName = path

		// TODO: consider the other places where ignores can come from, such as core.excludesFile and .git/info/exclude
	}

	return patternMatches, nil
}

func findPatternInGitIgnore(c *Client, gitignore string, wdpath string) (string, int, error) {
	_, err := os.Stat(gitignore)
	if os.IsNotExist(err) {
		return "", 0, nil
	}

	if err != nil {
		return "", 0, err
	}

	ignorefile, err := os.Open(gitignore)
	if err != nil {
		return "", 0, err
	}
	defer ignorefile.Close()

	reader := bufio.NewReader(ignorefile)
	lineNumber := 0

	for {
		pattern := ""
		isEof := false

		for {
			linebytes, isprefix, err := reader.ReadLine()
			pattern = pattern + string(linebytes)

			if isprefix {
				continue
			}

			lineNumber++

			if err == io.EOF {
				isEof = true
			}
			break
		}

		log.Printf("Checking pattern %s in %s\n", pattern, gitignore)
		if pattern == "" || strings.HasPrefix(pattern, "#") {
			if isEof {
				break
			}
			continue
		}

		matched := File(filepath.Base(wdpath)).MatchGlob(pattern)
		if err != nil {
			if isEof {
				break
			}
			continue // Problem with the pattern in the gitignore file
		}

		// TODO matches based on other segments of the path, not just the file name

		if matched {
			return pattern, lineNumber, nil
		}

		if isEof {
			break
		}
	}

	return "", 0, nil
}
