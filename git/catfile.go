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
	FollowSymlinks    bool
}

func catFilePretty(c *Client, obj GitObject, opts CatFileOptions) (string, error) {
	switch t := obj.GetType(); t {
	case "commit", "tree", "blob", "tag":
		if opts.Pretty {
			return obj.String(), nil
		}
		return string(obj.GetContent()), nil
	default:
		return "", fmt.Errorf("Invalid git type: %s", t)
	}
}
func CatFile(c *Client, typ string, s Sha1, opts CatFileOptions) (string, error) {
	if opts.FollowSymlinks {
		return "", fmt.Errorf("FollowSymlinks only valid in batch mode")
	}
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
		case "blob":
			switch o := obj.(type) {
			case GitBlobObject:
				return string(obj.GetContent()), nil
			case GitTagObject:
				if o.GetHeader("type") != "blob" {
					return "", fmt.Errorf("tag does not tag a blob")
				}
				tagged := o.GetHeader("object")
				s, err := Sha1FromString(tagged)
				if err != nil {
					panic(err)
					return "", err
				}

				return CatFile(c, typ, s, opts)
			}
			return "", fmt.Errorf("Invalid blob type")
		case "commit", "tree", "tag":
			return string(obj.GetContent()), nil
		default:
			return "", fmt.Errorf("invalid object type %v", typ)

		}
	}

}

func CatFileBatch(c *Client, opts CatFileOptions, r io.Reader, w io.Writer) error {
	if opts.Type || opts.Size || opts.ExitCode || opts.Pretty {
		return fmt.Errorf("May not combine options with --batch")
	}
	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		line := scanner.Text()
		var obj []ParsedRevision
		var err error
		var rest string
		var id string
		if strings.Contains(opts.BatchFmt, "%(rest)") {
			split := strings.Fields(line)
			id = split[0]
			rest = strings.TrimLeft(line[len(split[0]):], "\n \t\r")
		} else {
			id = line
		}
		if strings.TrimSpace(id) == "" {
			fmt.Fprintf(w, "%v missing\n", id)
			continue
		}
		obj, _ = RevParse(c, RevParseOptions{Quiet: true}, []string{id})
		if len(obj) == 0 {
			fmt.Fprintf(w, "%v missing\n", id)
			continue
		} else if len(obj) > 1 {
			fmt.Fprintf(w, "%v ambiguous\n", id)
			continue
		}
		gitobj, err := c.GetObject(obj[0].Id)
		if err != nil {
			if err.Error() == "Object not found." {
				fmt.Fprintf(w, "%v missing\n", id)
				continue
			}
			return err
		}

		if opts.BatchFmt != "" {
			str := opts.BatchFmt
			str = strings.Replace(str, "%(objectname)", obj[0].Id.String(), -1)
			str = strings.Replace(str, "%(objecttype)", gitobj.GetType(), -1)
			str = strings.Replace(str, "%(objectsize)", strconv.Itoa(gitobj.GetSize()), -1)
			str = strings.Replace(str, "%(rest)", rest, -1)
			fmt.Fprintln(w, str)
		} else {
			fmt.Fprintf(w, "%v %v %v\n", obj[0].Id, gitobj.GetType(), gitobj.GetSize())
		}
		if opts.Batch && !opts.BatchCheck {
			fmt.Fprintf(w, "%v\n", string(gitobj.GetContent()))
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}
