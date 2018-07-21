// +build dragonfly linux

package git

import (
	"syscall"
)

func (f File) CTime() (uint32, uint32) {
	stat, err := f.Lstat()
	if err != nil {
		return 0, 0
	}
	rawstat := stat.Sys().(*syscall.Stat_t)
	tspec := rawstat.Ctim
	return uint32(tspec.Sec), uint32(tspec.Nsec)
}
