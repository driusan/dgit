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
	NoIndex bool
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
		if !opts.NoIndex {
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
		}

		abs, err := filepath.Abs(path.String())
		if err != nil {
			return nil, err
		}

		stat, _ := os.Lstat(abs)
		isDir := false
		if stat != nil {
			isDir = stat.IsDir()
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
			ignorePath, err := filepath.Rel(dir, abs)
			pattern, lineNumber, err := findPatternInIgnoreFile(c, gitignore, ignorePath, isDir)
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

		// Check .git/info/exclude
		if patternMatches[idx].Pattern == "" {
			pattern, lineNumber, err := findPatternInIgnoreFile(c, filepath.Join(c.GitDir.String(), "info/exclude"), wdpath, isDir)
			if err != nil {
				return nil, err
			}

			if pattern != "" {
				patternMatches[idx] = IgnoreMatch{Pattern: pattern, LineNum: lineNumber, Source: File(".git/info/exclude"), PathName: path} // TODO .git/info/exclude is hard coded here instead of being calculated from c.GitDir
			}
		}

		// Be sure to assign the pathname in all cases so that clients can match the outputs to their inputs
		patternMatches[idx].PathName = path

		// TODO: consider the other places where ignores can come from, such as core.excludesFile and .git/info/exclude
	}

	return patternMatches, nil
}

// Finds the first matching pattern and line number in the provided ignoreFile that matches the ignorePath, relative to the
//  the ignoreFile, if one exists. If no pattern matches or the ignoreFile doesn't exist then the returned pattern is an empty string.
//  If the ignoreFile is not within the work directory then the ignorePath should be made relative to the work directory.
func findPatternInIgnoreFile(c *Client, ignoreFile string, ignorePath string, isDir bool) (string, int, error) {
	log.Printf("Searching for matching ignore pattern for %s in %s\n", ignorePath, ignoreFile)
	_, err := os.Stat(ignoreFile)
	if os.IsNotExist(err) {
		return "", 0, nil
	}

	if err != nil {
		return "", 0, err
	}

	file, err := os.Open(ignoreFile)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
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

		log.Printf("Checking pattern %s in %s\n", pattern, ignoreFile)
		if pattern == "" || strings.HasPrefix(pattern, "#") {
			if isEof {
				break
			}
			continue
		}

		matched := matchesGlob("/"+ignorePath, isDir, pattern)
		if matched {
			return pattern, lineNumber, nil
		}

		if isEof {
			break
		}
	}

	return "", 0, nil
}

func matchesGlob(path string, isDir bool, pattern string) bool {
	if !strings.HasPrefix(pattern, "/") {
		pattern = "/**/" + pattern
	}

	pathSegs := strings.Split(path, "/")
	patternSegs := strings.Split(pattern, "/")

	for len(pathSegs) > 0 && len(patternSegs) > 0 {
		if patternSegs[0] == "**" && len(patternSegs) > 1 {
			if m, _ := filepath.Match(patternSegs[1], pathSegs[0]); m {
				patternSegs = patternSegs[2:]
			}
		} else if m, _ := filepath.Match(patternSegs[0], pathSegs[0]); m {
			patternSegs = patternSegs[1:]
		} else {
			break
		}

		pathSegs = pathSegs[1:]
	}

	if len(patternSegs) == 0 {
		return true
	} else if patternSegs[0] == "" && len(pathSegs) > 0 {
		return true
	} else if patternSegs[0] == "" && isDir {
		return true
	} else {
		return false
	}
}
