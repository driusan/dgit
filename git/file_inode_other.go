// +build !dragonfly
// +build !darwin
// +build !linux
// +build !openbsd

package git

func (f File) INode() uint32 {
	return 0
}
