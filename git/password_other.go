//+build !plan9

package git

import (
	"fmt"
	"os"

	"golang.org/x/crypto/ssh/terminal"
)

func getUserPassword(url string) (userPasswd, error) {
	user := readLine("Username: ")

	fmt.Fprintf(os.Stderr, "Password: ")
	pwb, err := terminal.ReadPassword(0)
	return userPasswd{user, string(pwb)}, err
}
