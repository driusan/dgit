// +build !dragonfly
// +build !darwin
// +build !linux
// +build !openbsd
// +build !netbsd

package git

func (f File) INode() uint32 {
	return 0
}
