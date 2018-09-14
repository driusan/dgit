//+build !plan9

package git

import (
	"fmt"
	"os"

	"golang.org/x/crypto/ssh/terminal"
)

func getPassword(url string) (string, error) {
	fmt.Fprintf(os.Stderr, "Password: ")
	pwb, err := terminal.ReadPassword(0)
	return string(pwb), err
}
