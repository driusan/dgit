package git

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type CheckIgnoreOptions struct {
}

func CheckIgnore(c *Client, opts CheckIgnoreOptions, paths []File) ([]string, error) {
	patternMatches := make([]string, len(paths))

	for idx, path := range paths {
		log.Printf("Checking if %s is tracked by git\n", path.String())
		entries, err := LsFiles(c, LsFilesOptions{ExcludeStandard: false}, []File{path})
		if err != nil {
			return nil, err
		}

		// As a default, nothing that is tracked by git is ignored
		if len(entries) > 0 {
			log.Printf("Path %v is tracked by git and not ignored\n")
			patternMatches[idx] = ""
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
			log.Printf("Checking .gitignore in %s\n", dir)
			pattern, err := findPatternInGitIgnore(c, filepath.Join(dir, ".gitignore"), wdpath)
			if err != nil {
				return nil, err
			}

			if pattern != "" {
				patternMatches[idx] = pattern
				break
			}

			if dir == c.WorkDir.String() {
				break
			}
			dir = filepath.Dir(dir)
		}
	}

	return patternMatches, nil
}

func findPatternInGitIgnore(c *Client, gitignore string, wdpath string) (string, error) {
	_, err := os.Stat(gitignore)
	if os.IsNotExist(err) {
		return "", nil
	}

	if err != nil {
		return "", err
	}

	ignorefile, err := os.Open(gitignore)
	if err != nil {
		return "", err
	}
	defer ignorefile.Close()

	reader := bufio.NewReader(ignorefile)

	for {
		pattern := ""
		isEof := false

		for {
			linebytes, isprefix, err := reader.ReadLine()
			pattern = pattern + string(linebytes)

			if isprefix {
				continue
			}

			if err == io.EOF {
				log.Printf("Error is %v\n", err)
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

		matched, err := filepath.Match(pattern, filepath.Base(wdpath))
		if err != nil {
			if isEof {
				break
			}
			continue // Problem with the pattern in the gitignore file
		}

		// TODO matches based on other segments of the path, not just the file name

		if matched {
			return pattern, nil
		}

		if isEof {
			break
		}
	}

	return "", nil
}
