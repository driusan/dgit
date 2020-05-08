package git

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type CatFileOptions struct {
	Type, Size, Pretty bool
	ExitCode           bool
	AllowUnknownType   bool

	Batch, BatchCheck bool
	BatchFmt          string
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

func CatFileBatch(c *Client, opts CatFileOptions, r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		line := scanner.Text()
		obj, err := RevParse(c, RevParseOptions{}, []string{line})
		if err != nil {
			return err
		}
		gitobj, err := c.GetObject(obj[0].Id)
		if err != nil {
			return err
		}
		if opts.BatchFmt != "" {
			str := opts.BatchFmt
			str = strings.Replace(str, "%(objectname)", obj[0].Id.String(), -1)
			str = strings.Replace(str, "%(objecttype)", gitobj.GetType(), -1)
			str = strings.Replace(str, "%(objectsize)", strconv.Itoa(gitobj.GetSize()), -1)
			fmt.Fprintln(w, str)
		} else {
			fmt.Fprintf(w, "%v %v %v\n", obj[0].Id, gitobj.GetType(), gitobj.GetSize())
			if opts.Batch && !opts.BatchCheck {
				fmt.Fprintf(w, "%v\n", string(gitobj.GetContent()))
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}
