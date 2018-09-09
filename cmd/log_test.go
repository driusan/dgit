package cmd

import (
	"testing"

	"github.com/driusan/dgit/git"
)

func BenchmarkLogHead(b *testing.B) {
	c, err := git.NewClient("../.git", ".")
	if err != nil {
		panic(err)
	}
	for n := 0; n < b.N; n++ {
		if err := Log(c, []string{}); err != nil {
			panic(err)
		}
	}
}
