package git

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
)

type patchHunk struct {
	File IndexPath
	Hunk string
}

// Split a patch into the hunks which make up the patch.
func splitPatch(fullpatch string, nameonly bool) ([]patchHunk, error) {
	// Regexp to extract the different files that are part of the patch
	fileRE := regexp.MustCompile(`(?m)^diff --git a/([[:graph:]]+) b/([[:graph:]]+)$`)
	filechunks := fileRE.FindAllStringSubmatchIndex(fullpatch, -1)
	var ret []patchHunk
	for i, match := range filechunks {
		a := fullpatch[match[2]:match[3]]
		b := fullpatch[match[4]:match[5]]
		if a != b {
			return nil, fmt.Errorf("Filenames do not match")
		}
		var patch string
		if i == len(filechunks)-1 {
			patch = fullpatch[match[0]:]
		} else {
			patch = fullpatch[match[0]:filechunks[i+1][0]]
		}
		if nameonly {
			ret = append(ret, patchHunk{IndexPath(a), ""})

		} else {
			pieces := extractPatchHunks(IndexPath(a), patch)
			ret = append(ret, pieces...)
		}
	}
	return ret, nil
}

func extractPatchHunks(name IndexPath, filepatch string) []patchHunk {
	// Regex to extract the hunk header which delineates different hunks
	hunkRE := regexp.MustCompile(`(?m)^@@ -([\d]+)(?:,[\d]+)? \+([\d]+)(?:,[\d]+)? @@$`)

	// Regex to extract parts of that hunk that are actually part of the patch.
	// Must start with a space, a plus, or a minus sign (for context diff)
	hunks := hunkRE.FindAllStringIndex(filepatch, -1)
	var ret []patchHunk
	for i, match := range hunks {
		var hunk string
		if i == len(hunks)-1 {
			hunk = filepatch[match[0]:]
		} else {
			hunk = filepatch[match[0]:hunks[i+1][0]]
		}
		ret = append(ret, patchHunk{name, hunk})
	}
	return ret
}

func recombinePatch(w io.Writer, hunks []patchHunk) {
	var lastPath IndexPath
	for _, hunk := range hunks {
		if lastPath != hunk.File {
			printDiffHeader(w, hunk.File, true)
		}
		fmt.Fprint(w, hunk.Hunk)
	}
}

var userAborted = errors.New("User aborted action")

func filterHunks(prompt string, patch []patchHunk) ([]patchHunk, error) {
	var filtered []patchHunk
	var lastPath IndexPath
	for _, hunk := range patch {
		if lastPath != hunk.File {
			printDiffHeader(os.Stdout, hunk.File, true)
		}
		fmt.Print(hunk.Hunk)
		scanner := bufio.NewScanner(os.Stdin)
		fmt.Printf("%v (y/n/q/?) ", prompt)
	promptLoop:
		for scanner.Scan() {
			switch scanner.Text() {
			case "y":
				filtered = append(filtered, hunk)
				break promptLoop
			case "n":
				break promptLoop
			case "q":
				return nil, userAborted
			case "?":
				fmt.Fprintf(os.Stderr, fmt.Sprintf(`y - %v
n - do not %v
q - quit; do not %v or any of the remaining ones
? - print help
`, prompt, prompt, prompt))
			}
			fmt.Printf("%v (y/n/q/?) ", prompt)

		}
	}
	return filtered, nil
}
