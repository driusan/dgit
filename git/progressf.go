// +build !plan9

package git

import (
	"fmt"
	"os"
)

// Print progress information to stderr
func progressF(fmtS string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "\r"+fmtS, args...)
}
