package git

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type InitOptions struct {
	Quiet bool
	Bare  bool

	Template File

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

	reinit := false
	if !opts.Bare {
		if File(dir + "/.git").Exists() {
			reinit = true
		} else if err := os.Mkdir(dir+"/.git", 0755); err != nil {
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

	bareConf := "bare = false"
	if opts.Bare {
		bareConf = "bare = true"
	}

	if c.GitDir.File("HEAD").Exists() {
		reinit = true
	} else if err := c.GitDir.WriteFile("HEAD", []byte("ref: refs/heads/master\n"), 0644); err != nil {
		return nil, err
	}

	if c.GitDir.File("config").Exists() {
		reinit = true
	} else if err := c.GitDir.WriteFile("config", []byte("[core]\n\trepositoryformatversion = 0\n\t"+bareConf+"\n"), 0644); err != nil {
		return nil, err
	}
	if c.GitDir.File("description").Exists() {
		reinit = true
	} else if err := c.GitDir.WriteFile("description", []byte("Unnamed repository; edit this file 'description' to name the repository.\n"), 0644); err != nil {
		return nil, err
	}
	if !opts.Quiet {
		dir, err := filepath.Abs(c.GitDir.String())
		if err != nil {
			return c, err
		}
		if reinit {
			fmt.Printf("Reinitialized existing Git repository in %v/\n", dir)
		} else {
			fmt.Printf("Initialized empty Git repository in %v/\n", dir)
		}
	}

	// Now go into the directory and adjust workdir and gitdir so that
	// tests are in the right place.
	if !opts.Bare {
		if err := os.Chdir(c.WorkDir.String()); err != nil {
			return c, err
		}
		wd, err := os.Getwd()
		if err != nil {
			return c, err
		}
		c.WorkDir = WorkDir(wd)
		c.GitDir = GitDir(wd + "/.git")
	}

	if opts.Template != "" {
		err := filepath.Walk(opts.Template.String(), func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			path, err = filepath.Rel(opts.Template.String(), path)
			if err != nil {
				return err
			}

			if info.IsDir() {
				os.MkdirAll(filepath.Join(c.GitDir.String(), path), 0777)
			} else {
				if c.GitDir.File(File(path)).Exists() {
					return nil
				}
				newFile, err := c.GitDir.Create(File(path))
				if err != nil {
					return err
				}
				defer newFile.Close()
				f, err := os.Open(filepath.Join(opts.Template.String(), path))
				if err != nil {
					return err
				}
				defer f.Close()
				_, err = io.Copy(newFile, f)
				if err != nil {
					return err
				}
			}

			return nil
		})

		return c, err
	}

	return c, nil
}
