// +build !plan9

package git

const (
	// the command to execute for a posix compliant diff implementation.
	posixDiff  = "diff"
	posixDiff3 = "diff3"

	posixPatch = "patch"
)
