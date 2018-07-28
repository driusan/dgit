package git

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// An ignore pattern declared in a pattern file (e.g. .gitignore) or provided as an input.
type IgnorePattern struct {
	Pattern string // The pattern to match, relative to the scope
	Source  File   // The path to the pattern file where this pattern is found, relative path for work dir resources, otherwise absolute path or blank for one that was provided as input to dgit
	Scope   File   // The work directory relative scope that this pattern applies
	LineNum int    // The line number within the source pattern file where this pattern is located
}

// Represents a match (or non-match) of a particular path name against a set of ignore patterns
type IgnoreMatch struct {
	IgnorePattern      // A match has all of the pattern information, or empty pattern if no match was found
	PathName      File // The provided path name that was checked for the ignore
}

// Returns the standard representation of an ignore match (or non-match)
func (im IgnoreMatch) String() string {
	return fmt.Sprintf("%s:%s:%s\t%s", im.Source, im.LineString(), im.Pattern, im.PathName)
}

func (im IgnoreMatch) LineString() string {
	lineNum := ""
	if im.LineNum != 0 {
		lineNum = strconv.Itoa(im.LineNum)
	}
	return lineNum
}

func (ip IgnorePattern) Matches(ignorePath string, isDir bool) bool {
	if !strings.HasPrefix(ignorePath, ip.Scope.String()) {
		return false
	}

	rel, err := filepath.Rel("/"+ip.Scope.String(), "/"+ignorePath)

	// Not in scope of this pattern
	if err != nil || strings.HasPrefix(rel, ".") {
		return false
	}

	return matchesGlob("/"+rel, isDir, ip.Pattern)
}

// Returns the standard ignores (.gitignore files and various global sources) that
//  should be used to determine whether the provided files are ignored. These patterns
//  can be used with these files with ApplyIgnore() to find whether each file is ignored
//  or not.
func StandardIgnorePatterns(c *Client, paths []File) ([]IgnorePattern, error) {
	alreadyParsed := make(map[string]bool)
	finalPatterns := []IgnorePattern{}

	for _, path := range paths {
		abs, err := filepath.Abs(path.String())
		if err != nil {
			return nil, err
		}

		// Let's check that this path is in the git work dir first
		wdpath, err := filepath.Rel(c.WorkDir.String(), abs)
		if err != nil || strings.HasPrefix(wdpath, "..") {
			return nil, fmt.Errorf("Path %v is not in the git work directory.", path.String())
		}

		// this could be the work dir specified as '.' so let's start with that.
		dir := filepath.Dir(abs)

		for {
			gitignore := filepath.Join(dir, ".gitignore")
			if _, ok := alreadyParsed[gitignore]; ok {
				if dir == c.WorkDir.String() {
					break
				}
				dir = filepath.Dir(dir)
				continue
			}
			alreadyParsed[gitignore] = true

			scope, err := filepath.Rel(c.WorkDir.String(), dir)
			if err != nil {
				return []IgnorePattern{}, err
			}

			if scope == "." {
				scope = ""
			}

			ignorePatterns, err := ParseIgnorePatterns(c, File(gitignore), File(scope))
			if err != nil {
				return []IgnorePattern{}, err
			}

			finalPatterns = append(finalPatterns, ignorePatterns...)

			if dir == c.WorkDir.String() || dir == filepath.Dir(c.WorkDir.String()) {
				break
			}
			dir = filepath.Dir(dir)
		}
	}

	ignorePatterns, err := ParseIgnorePatterns(c, File(filepath.Join(c.GitDir.String(), "info/exclude")), File(""))
	if err != nil {
		return []IgnorePattern{}, err
	}

	finalPatterns = append(finalPatterns, ignorePatterns...)

	return finalPatterns, err
}

func ParseIgnorePatterns(c *Client, patternFile File, scope File) ([]IgnorePattern, error) {
	log.Printf("Parsing %s for ignore patterns\n", patternFile)
	_, err := patternFile.Lstat()
	if os.IsNotExist(err) {
		return []IgnorePattern{}, nil
	}

	if err != nil {
		return []IgnorePattern{}, err
	}

	file, err := patternFile.Open()
	if err != nil {
		return []IgnorePattern{}, err
	}
	defer file.Close()

	ignorePatterns := []IgnorePattern{}
	reader := bufio.NewReader(file)
	lineNumber := 0
	source := patternFile
	rel, err := filepath.Rel(c.WorkDir.String(), patternFile.String())
	if err == nil {
		source = File(rel)
	}

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

		if pattern != "" && !strings.HasPrefix(pattern, "#") {
			lastEscape := strings.LastIndex(pattern, "\\") + 1
			if len(pattern) > lastEscape {
				pattern = pattern[:lastEscape+1] + strings.TrimSpace(pattern[lastEscape+1:])
			}

			pattern = strings.Replace(pattern, "\\#", "#", 1)
			pattern = strings.Replace(pattern, "\\ ", " ", -1)
			pattern = strings.Replace(pattern, "\\\t", "\t", -1)

			ignorePattern := IgnorePattern{Pattern: pattern, LineNum: lineNumber, Scope: scope, Source: source}
			ignorePatterns = append(ignorePatterns, ignorePattern)
		}

		if isEof {
			break
		}
	}

	return ignorePatterns, nil
}

// Patterns are sorted by their scope so that the most specific scopes are first and the more
//  broad scopes are later on. This is to allow broader scopes to negate previous pattern matches
//  in the future.
func sortPatterns(patterns *[]IgnorePattern) {
	sort.Slice(*patterns, func(i, j int) bool {
		return len((*patterns)[i].Scope.String()) > len((*patterns)[i].Scope.String())
	})
}

func MatchIgnores(c *Client, patterns []IgnorePattern, paths []File) ([]IgnoreMatch, error) {
	log.Printf("Matching ignores for paths %v using patterns %+v\n", paths, patterns)
	patternMatches := make([]IgnoreMatch, len(paths))

	sortPatterns(&patterns)

	for idx, path := range paths {
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
		if err != nil || strings.HasPrefix(wdpath, "..") {
			return nil, fmt.Errorf("Path %v is not in the git work directory.", path.String())
		}

		for _, pattern := range patterns {
			if pattern.Matches(wdpath, isDir) {
				patternMatches[idx].Pattern = pattern.Pattern
				patternMatches[idx].Source = pattern.Source
				patternMatches[idx].Scope = pattern.Scope
				patternMatches[idx].LineNum = pattern.LineNum
				break // For now, until we support negation
			}
		}

		// Be sure to assign the pathname in all cases so that clients can match the outputs to their inputs
		patternMatches[idx].PathName = path
	}

	return patternMatches, nil
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
