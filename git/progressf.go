//go:build !plan9
// +build !plan9

package git

import (
	"fmt"
	"os"
	"time"
)

var lastProgress int64

// Print progress information to stderr
func progressF(fmtS string, done bool, args ...interface{}) {
	if done {
		fmt.Fprintf(os.Stderr, "\r"+fmtS+", done\n", args...)
	}
	now := time.Now().Unix()
	if lastProgress > 0 && now-lastProgress < 3 {
		return
	}
	fmt.Fprintf(os.Stderr, "\r"+fmtS, args...)
	lastProgress = now
}
