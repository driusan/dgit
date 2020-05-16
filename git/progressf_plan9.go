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
		fmt.Fprintf(os.Stderr, "\n"+fmtS+", done\n", args...)
	}
	now := time.Now().Unix()
	if lastProgress > 0 && now-lastProgress < 3 {
		return
	}
	fmt.Fprintf(os.Stderr, "\n"+fmtS, args...)
	lastProgress = now
}
