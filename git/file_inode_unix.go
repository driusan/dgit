//go:build dragonfly || linux || openbsd || netbsd
// +build dragonfly linux openbsd netbsd

package git

import (
	"syscall"
)

func (f File) INode() uint32 {
	stat, err := f.Lstat()
	if err != nil {
		return 0
	}
	rawstat := stat.Sys().(*syscall.Stat_t)
	return uint32(rawstat.Ino)
}
