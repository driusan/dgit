// +build !dragonfly
// +build !darwin
// +build !linux

package git

func (f File) Inode() uint32 {
	return 0
}
