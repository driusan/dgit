package cmd

import (
	"testing"

	"github.com/driusan/dgit/git"
)

func BenchmarkRevListHead(b *testing.B) {
	c, err := git.NewClient("../.git", ".")
	if err != nil {
		panic(err)
	}
	for n := 0; n < b.N; n++ {
		if err := RevList(c, []string{"--quiet", "HEAD"}); err != nil {
			panic(err)
		}
	}
}

func BenchmarkRevListHeadExclude(b *testing.B) {
	c, err := git.NewClient("../.git", ".")
	if err != nil {
		panic(err)
	}
	for n := 0; n < b.N; n++ {
		if err := RevList(c, []string{"--quiet", "HEAD", "^HEAD^"}); err != nil {
			panic(err)
		}
	}
}

func BenchmarkRevListHeadExcludeObjects(b *testing.B) {
	c, err := git.NewClient("../.git", ".")
	if err != nil {
		panic(err)
	}
	for n := 0; n < b.N; n++ {
		if err := RevList(c, []string{"--quiet", "--objects", "HEAD", "^HEAD^"}); err != nil {
			panic(err)
		}
	}
}
