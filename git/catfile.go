package git

import (
	"fmt"
)

type CatFileOptions struct {
	Type, Size, Pretty bool
}

func catFilePretty(c *Client, obj GitObject, opts CatFileOptions) (string, error) {
	switch t := obj.GetType(); t {
	case "commit", "tree", "blob":
		return obj.String(), nil
	case "tag":
		return "", fmt.Errorf("-p tag not yet implemented")
	default:
		return "", fmt.Errorf("Invalid git type: %s", t)
	}
}
func CatFile(c *Client, s Sha1, opts CatFileOptions) (string, error) {
	obj, err := c.GetObject(s)
	if err != nil {
		return "", err
	}

	switch {
	case opts.Pretty:
		return catFilePretty(c, obj, opts)
	case opts.Type:
		return obj.GetType(), nil
	case opts.Size:
		return fmt.Sprintf("%v", obj.GetSize), nil
	default:
		return "", fmt.Errorf("Not yet implemented.")
	}

}
