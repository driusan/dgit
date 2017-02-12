package cmd

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/driusan/dgit/git"
)

func PackObjects(c *git.Client, input io.Reader, args []string) {
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s pack-objects basename\n", os.Args[0])
		return
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
