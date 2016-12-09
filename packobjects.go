package main

import (
	"bufio"
	"encoding/hex"
	"fmt"
	libgit "github.com/driusan/git"
	"io"
	"os"
)

func PackObjects(repo *libgit.Repository, input io.Reader, args []string) {
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s pack-objects basename\n", os.Args[0])
		return
	}
	f, _ := os.Create(args[0] + ".pack")
	defer f.Close()

	var objects []Sha1
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		b, err := hex.DecodeString(scanner.Text())
		if err != nil {
			panic(err)
		}
		s, err := Sha1FromSlice(b)
		if err != nil {
			panic(err)
		}
		objects = append(objects, s)
	}
	SendPackfile(repo, f, objects)
	//SendPackfile(f, []Sha1{})
}
