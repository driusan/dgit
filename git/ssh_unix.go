// +build dragonfly openbsd darwin freebsd netbsd solaris linux

package git

import (
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"os/user"
)

func getSigners() ([]ssh.Signer, error) {
	user, err := user.Current()
	if err != nil {
		return nil, nil
	}
	f, err := ioutil.ReadFile(user.HomeDir + "/.ssh/id_rsa")
	if err != nil {
		return nil, nil
	}
	key, err := ssh.ParsePrivateKey(f)
	if err != nil {
		return nil, err
	}
	return []ssh.Signer{key}, nil
}
