package cmd

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"../git"
)

func getLogVar(c *git.Client, config *git.GitConfig, key string, envVar string) string {
	if envVar != "" {
		return envVar
	}

	if key == "GIT_AUTHOR_IDENT" {
		name := os.Getenv("GIT_AUTHOR_NAME")
		email := os.Getenv("GIT_AUTHOR_EMAIL")
		date := os.Getenv("GIT_AUTHOR_DATE")
		if name != "" && email != "" && date != "" {
			return fmt.Sprintf("%s <%s> %s", name, email, date)
		}
	}

	if key == "GIT_COMMITTER_IDENT" {
		name := os.Getenv("GIT_COMMITTER_NAME")
		email := os.Getenv("GIT_COMMITTER_EMAIL")
		date := os.Getenv("GIT_COMMITTER_DATE")
		if name != "" && email != "" && date != "" {
			return fmt.Sprintf("%s <%s> %s", name, email, date)
		}
	}

	if key == "GIT_AUTHOR_IDENT" || key == "GIT_COMMITTER_IDENT" {
		t := time.Now()
		person := c.GetAuthor(&t)
		return fmt.Sprintf("%s <%s> %v %s", person.Name, person.Email, time.Now().Unix(), time.Now().Format("-0700"))
	} else if key == "GIT_EDITOR" {
		coreEditor, _ := config.GetConfig("core.editor")
		if coreEditor != "" {
			return coreEditor
		}

		visual := os.Getenv("VISUAL")
		if visual != "" {
			return visual
		}

		editor := os.Getenv("EDITOR")
		if editor != "" {
			return editor
		}

		return "ed"
	} else if key == "GIT_PAGER" {
		corePager, _ := config.GetConfig("core.pager")
		if corePager != "" {
			return corePager
		}

		pager := os.Getenv("PAGER")
		if pager != "" {
			return pager
		}
		return "less"
	}

	return ""
}

func Var(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("var", flag.ExitOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.Usage = func() {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n\nOptions:\n")
		flags.PrintDefaults()
	}

	list := flags.Bool("l", false, "List all logical variables along with their values")

	flags.Parse(args)

	if *list && flags.NArg() > 0 {
		fmt.Fprintf(flag.CommandLine.Output(), "Cannot specify both the -l parameter and a variable name\n\n")
		flags.Usage()
		os.Exit(1)
	} else if flags.NArg() > 1 {
		flags.Usage()
		os.Exit(1)
	}

	fname := c.GitDir.String() + "/config"

	file, err := os.OpenFile(fname, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	config := git.ParseConfig(file)
	err = file.Close()
	if err != nil {
		return err
	}

	switch {
	case flags.NArg() == 1:
		fmt.Printf("%s\n", getLogVar(c, &config, flags.Arg(0), os.Getenv(flags.Arg(0))))
		return nil
	case *list:
		list := config.GetConfigList()
		for _, entry := range list {
			fmt.Printf("%s\n", entry)
		}
		for _, logVar := range []string{"GIT_AUTHOR_IDENT", "GIT_COMMITTER_IDENT", "GIT_EDITOR", "GIT_PAGER"} {
			fmt.Printf("%s=%s\n", logVar, getLogVar(c, &config, logVar, os.Getenv(logVar)))
		}
		return nil
	}

	flags.Usage()
	os.Exit(2)

	return errors.New("Unhandled action")
}
