package git

import (
	"fmt"
	"os"
	"path/filepath"
)

type InitOptions struct {
	Quiet bool
	Bare  bool

	// Not implemented
	TemplateDir File

	// Not implemented
	SeparateGitDir File

	// Not implemented
	Shared os.FileMode
}

func Init(c *Client, opts InitOptions, dir string) (*Client, error) {
	if dir == "" {
		dir2, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		dir = dir2
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	if err := os.Chdir(dir); err != nil {
		return nil, err
	}

	if !opts.Bare {
		if err := os.Mkdir(dir+"/.git", 0755); err != nil {
			return nil, err
		}
		if c != nil {
			c.GitDir = GitDir(dir + "/.git")
			c.WorkDir = WorkDir(dir)
		} else {
			c2, err := NewClient(dir+"/.git", dir)
			if err != nil {
				return nil, err
			}
			c = c2
		}
	} else {
		if c != nil {
			c.GitDir = GitDir(dir)
		} else {
			c2, err := NewClient(dir, "")
			if err != nil {
				return nil, err
			}
			c = c2
		}
	}

	// These are all the directories created by a clean "git init"
	// with the canonical git implementation
	if err := os.MkdirAll(c.GitDir.String()+"/objects/pack", 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(c.GitDir.String()+"/objects/info", 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(c.GitDir.String()+"/info", 0755); err != nil {
		// FIXME: Should have exclude file in it
		return nil, err
	}
	if err := os.MkdirAll(c.GitDir.String()+"/hooks", 0755); err != nil {
		// FIXME: Should have sample hooks in it.
		return nil, err
	}
	if err := os.MkdirAll(c.GitDir.String()+"/branches", 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(c.GitDir.String()+"/refs/heads", 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(c.GitDir.String()+"/refs/tags", 0755); err != nil {
		return nil, err
	}
	if err := c.GitDir.WriteFile("HEAD", []byte("ref: refs/heads/master\n"), 0644); err != nil {
		return nil, err
	}
	if err := c.GitDir.WriteFile("config", []byte("[core]\n\trepositoryformatversion = 0\n\tbare = false\n"), 0644); err != nil {
		return nil, err
	}
	if err := c.GitDir.WriteFile("description", []byte("Unnamed repository; edit this file 'description' to name the repository.\n"), 0644); err != nil {
		return nil, err
	}
	if !opts.Quiet {
		dir, err := filepath.Abs(c.GitDir.String())
		if err != nil {
			return c, err
		}
		fmt.Printf("Initialized empty Git repository in %v/\n", dir)
	}
	return c, nil
}
