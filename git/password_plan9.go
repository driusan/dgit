package git

import (
	"bitbucket.org/mischief/libauth"
)

func getUserPassword(url string) (userPasswd, error) {
	val, err := libauth.Getuserpasswd(
		"proto=pass service=git role=client server=%s",
		url,
	)
	if err != nil {
		return userPasswd{}, err
	}
	return userPasswd{val.User, val.Password}, nil
}
