package git

import (
	"bitbucket.org/mischief/libauth"
	"os/user"
)

func getPassword(url string) (string, error) {
	user, err := user.Current()
	if err != nil {
		return "", err
	}

	return libauth.Getuserpasswd(
		"proto=pass service=git role=client server=%s user=%s",
		url,
		user.Username,
	)
}
