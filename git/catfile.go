package git

import (
	"fmt"
)

type CatFileOptions struct {
	Type, Size, Pretty bool
	ExitCode           bool
	AllowUnknownType   bool
}

func catFilePretty(c *Client, obj GitObject, opts CatFileOptions) (string, error) {
	switch t := obj.GetType(); t {
	case "commit", "tree", "blob":
		if opts.Pretty {
			return obj.String(), nil
		}
		return string(obj.GetContent()), nil
	case "tag":
		return "", fmt.Errorf("-p tag not yet implemented")
	default:
		return "", fmt.Errorf("Invalid git type: %s", t)
	}
}
func CatFile(c *Client, typ string, s Sha1, opts CatFileOptions) (string, error) {
	obj, err := c.GetObject(s)
	if err != nil {
		return "", err
	}

	switch {
	case opts.ExitCode:
		// If it was invalid, GetObject would have failed.
		return "", nil
	case opts.Pretty:
		return catFilePretty(c, obj, opts)
	case opts.Type:
		return obj.GetType(), nil
	case opts.Size:
		return fmt.Sprintf("%v", obj.GetSize()), nil
	default:
		switch typ {
		case "commit", "tree", "blob":
			return string(obj.GetContent()), nil
		default:
			return "", fmt.Errorf("invalid object type %v", typ)

		}
	}

}
