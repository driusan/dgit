package cmd

import (
	"bufio"
	"encoding/hex"
	"flag"
	"io"
	"os"

	"github.com/driusan/dgit/git"
)

func PackObjects(c *git.Client, input io.Reader, args []string) {
	if len(args) == 1 && args[0] == "--help" {
		flag.Usage()
		os.Exit(0)
	}

	if len(args) != 1 {
		flag.Usage()
		os.Exit(2)
	}

	f, _ := os.Create(args[0] + ".pack")
	defer f.Close()

	var objects []git.Sha1
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		b, err := hex.DecodeString(scanner.Text())
		if err != nil {
			panic(err)
		}
		s, err := git.Sha1FromSlice(b)
		if err != nil {
			panic(err)
		}
		objects = append(objects, s)
	}
	git.SendPackfile(c, f, objects)
}
