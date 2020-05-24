// +build !plan9
// +build !dragonfly
// +build !openbsd
// +build !darwin
// +build !freebsd
// +build !netbsd
// +build !solaris
// +build !linux

package git

import (
	"golang.org/x/crypto/ssh"
)

func getSigners() ([]ssh.Signer, error) {
	// By default, assume no keys
	return nil, nil
}
