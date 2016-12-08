package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

func (c *Client) GetAuthor() string {
	home := os.Getenv("HOME")
	if home == "" {
		home = os.Getenv("home") // On some OSes, it is home
	}
	configFile, err := os.Open(home + "/.gitconfig")
	config := parseConfig(configFile)
	if err != nil {
		panic(err)
	}

	name := config.GetConfig("user.name")
	email := config.GetConfig("user.email")
	return fmt.Sprintf("%s <%s>", name, email)

}
func CommitTree(c *Client, args []string) (string, error) {
	content := bytes.NewBuffer(nil)

	var parents []string
	var messageString, messageFile string
	var skipNext bool
	var tree string
	for idx, val := range args {
		if idx == 0 && val[0] != '-' {
			tree = val
			continue
		}

		if skipNext == true {
			skipNext = false
			continue
		}
		switch val {
		case "-p":
			parents = append(parents, args[idx+1])
			skipNext = true
		case "-m":
			messageString += "\n" + args[idx+1] + "\n"
			skipNext = true
		case "-F":
			messageFile = args[idx+1]
			skipNext = true

		}
	}
	if messageString == "" && messageFile == "" {
		// No -m or -F provided, read from STDIN
		m, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			panic(err)
		}
		messageString = "\n" + string(m)
	} else if messageString == "" && messageFile != "" {
		// No -m, but -F was provided. Read from file passed.
		m, err := ioutil.ReadFile(messageFile)
		if err != nil {
			panic(err)
		}
		messageString = "\n" + string(m)
	}

	lines := strings.Split(messageString, "\n")
	var strippedLines []string
	for _, line := range lines {
		if len(line) >= 1 && line[0] == '#' {
			continue
		}
		strippedLines = append(strippedLines, line)
	}
	messageString = strings.Join(strippedLines, "\n")
	if strings.TrimSpace(messageString) == "" {
		return "", fmt.Errorf("Aborting due to empty commit message")
	}

	if tree == "" {
		tree = args[len(args)-1]
	}
	// TODO: Validate tree id
	fmt.Fprintf(content, "tree %s\n", tree)
	for _, val := range parents {
		fmt.Fprintf(content, "parent %s\n", val)
	}

	author := c.GetAuthor()
	t := time.Now()
	_, tzoff := t.Zone()
	// for some reason t.Zone() returns the timezone offset in seconds
	// instead of hours, so convert it to an hour format string
	tzStr := fmt.Sprintf("%+03d00", tzoff/(60*60))
	fmt.Fprintf(content, "author %s %d %s\n", author, t.Unix(), tzStr)
	fmt.Fprintf(content, "committer %s %d %s\n", author, t.Unix(), tzStr)
	fmt.Fprintf(content, "%s", messageString)
	fmt.Printf("%s", content.Bytes())
	sha1, err := c.WriteObject("commit", content.Bytes())
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s", sha1), nil
}
