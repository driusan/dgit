package git

import (
	"syscall"
)

func (f File) MTime() (int64, error) {
	stat, err := f.Stat()
	if err != nil {
		return 0, err
	}
	// Plan 9 only has 1 second precision and pads the nanoseconds
	// with 0, so we pretend that the qid.Version is the number
	// of nanoseconds.
	base := int64(stat.ModTime().Unix()) << 32
	sys := stat.Sys()
	if internal, ok := sys.(*syscall.Dir); ok {
		return base | int64(internal.Qid.Vers), nil
		return int64(internal.Qid.Path), nil

	}
	panic("Could not get QID.path for file")

}
