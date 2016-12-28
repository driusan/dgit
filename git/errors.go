package git

import (
	"errors"
)

var InvalidHead error = errors.New("Invalid HEAD")
var InvalidBranch error = errors.New("Invalid branch")
var InvalidCommit error = errors.New("Invalid commit")
var InvalidTree error = errors.New("Invalid tree")
