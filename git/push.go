package git

import (
	"fmt"
	"os"
)

type PushOptions struct {
	SendPackOptions

	SetUpstream bool
}

func Push(c *Client, opts PushOptions, r Remote, refs []Refname) error {
	if len(refs) < 1 {
		// The cmd package should have validated this or
		// provided the defaults otherwise
		return fmt.Errorf("Must specify refnames to push")
	}
	if r == "" {
		return fmt.Errorf("Must specify remote to push to")
	}
	remoteurl, err := r.PushURL(c)
	if err != nil {
		return err
	}
	if remoteurl == "" {
		return fmt.Errorf("Could not determine URL for %v", r)
	}

	var sendrefs []Refname
	var config GitConfig
	if opts.SetUpstream {
		// Only bother parsing the config file if we're going
		// to modify it
		config, err = LoadLocalConfig(c)
		if err != nil {
			return err
		}
	}
	for _, ref := range refs {
		// Convert from the name given on the command line
		// to the remote ref in the config
		// Use the part after the ':' if specified
		name := string(ref.RemoteName())

		var merge string
		if opts.SetUpstream {
			config.SetConfig(
				fmt.Sprintf("branch.%v.remote", name),
				r.String(),
			)
			merge = "refs/heads/" + name
			config.SetConfig(
				fmt.Sprintf("branch.%v.merge", name),
				merge,
			)
		} else {
			merge = c.GetConfig("branch." + name + ".merge")
			if merge == "" {
				return fmt.Errorf("The branch %v has no upstream set.\n"+
					"To push and set the upstream to the remote named \"origin\" use:\n\n"+
					"\t%v push --set-upstream origin %v\n",
					name, os.Args[0], name)
			}
		}
		sendrefs = append(sendrefs, ref.LocalName()+":"+Refname(merge))
	}

	if opts.SetUpstream {
		if err := config.WriteConfig(); err != nil {
			return err
		}
	}

	return SendPack(c, opts.SendPackOptions, r, sendrefs)
}
