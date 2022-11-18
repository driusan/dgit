//go:build !plan9
// +build !plan9

package git

func (f File) MTime() (int64, error) {
	stat, err := f.Lstat()
	if err != nil {
		return 0, err
	}
	base := int64(stat.ModTime().Unix()) << 32
	return base | int64(stat.ModTime().Nanosecond()), nil
}
