//go:build !plan9 && !dragonfly && !openbsd && !darwin && !freebsd && !netbsd && !solaris && !linux
// +build !plan9,!dragonfly,!openbsd,!darwin,!freebsd,!netbsd,!solaris,!linux

package git

import (
	"golang.org/x/crypto/ssh"
)

func getSigners() ([]ssh.Signer, error) {
	// By default, assume no keys
	return nil, nil
}
