// +build !dragonfly
// +build !darwin
// +build !linux
// +build !openbsd
// +build !netbsd

package git

// Ctime returns the CTime and CTimeNano parts of a time struct
// for the ctime of this file. On systems where either there's no
// ctime, we don't know how to get it, or or we haven't implemented
// it yet, this will return 0s instead of an error so that there will
// be sane behaviour on places that use it (such as index updating)
// that we shouldn't be propagating a non-critical error.
func (f File) CTime() (uint32, uint32) {
	return 0, 0
}
