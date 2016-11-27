package main

import (
	"fmt"
)

type Sha1 []byte

func (s Sha1) String() string {
	if s == nil {
		return ""
	}
	return fmt.Sprintf("%x", string(s))
}
