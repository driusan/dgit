package git

import (
	"syscall"
)

func (f File) INode() uint32 {
	stat, err := f.Lstat()
	if err != nil {
		return 0, 0
	}
	rawstat := stat.Sys().(*syscall.Stat_t)
	return int32(rawstat.Ino)
}
