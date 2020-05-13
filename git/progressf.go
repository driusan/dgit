// +build !plan9

package git

import (
	"fmt"
	"os"
	"time"
)

var lastProgress int64

// Print progress information to stderr
func progressF(fmtS string, args ...interface{}) {
	now := time.Now().Unix()
	if lastProgress > 0 && now-lastProgress < 3 {
		return
	}
	lastProgress = now
	fmt.Fprintf(os.Stderr, "\r"+fmtS, args...)
}
