//go:build !dragonfly && !darwin && !linux && !openbsd && !netbsd
// +build !dragonfly,!darwin,!linux,!openbsd,!netbsd

package git

func (f File) INode() uint32 {
	return 0
}
