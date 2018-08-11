// +build !dragonfly
// +build !darwin
// +build !linux

package git

func (f File) INode() uint32 {
	return 0
}
