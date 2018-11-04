package git

import (
	"archive/tar"
	"archive/zip"
	"compress/flate"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

type ArchiveOptions struct {
	Verbose            bool
	List               bool
	WorktreeAttributes bool
	ArchiveFormat      string
	BasePrefix         string
	OutputFile         string
	CompressionLevel   int
}

type ArchiveFormat int

const (
	archiveTar = ArchiveFormat(iota)
	archiveTarGzip
	archiveZip
)

var supportedArchiveFormats = map[string]ArchiveFormat{
	"tar":    archiveTar,
	"tgz":    archiveTarGzip,
	"tar.gz": archiveTarGzip,
	"zip":    archiveZip,
}

func findOutputFileFormat(output string) (ArchiveFormat, error) {
	// if the output is empty return the default value, a tarball.
	if output == "" {
		return archiveTar, nil
	}

	for k, v := range supportedArchiveFormats {
		if strings.HasSuffix(strings.ToLower(output), k) {
			return v, nil
		}
	}
	// The archive format is not found,
	// return tar by default and an error.
	return archiveTar, errors.New("Archive format not supported!")
}

func createTarArchive(c *Client, opts ArchiveOptions, tgz bool, sha Sha1, entries []*IndexEntry) error {
	fileOutput := os.Stdout

	// If the output file is not empty use it instead of stdout
	if opts.OutputFile != "" {
		if file, err := os.Create(opts.OutputFile); err == nil {
			fileOutput = file
			defer file.Close()
		} else {
			return err
		}
	}

	mtime := time.Now()

	// If the sha is a tree we set the file modification time to the current time
	// otherwise we must use the commit time.
	if sha.Type(c) != "tree" {
		if t, err := CommitID(sha).GetDate(c); err == nil {
			mtime = t
		}
	}

	var tw *tar.Writer

	//
	if tgz {
		gw := gzip.NewWriter(fileOutput)
		defer gw.Close()

		tw = tar.NewWriter(gw)
		defer tw.Close()
	} else {
		tw = tar.NewWriter(fileOutput)
		defer tw.Close()
	}

	// Write the pax header
	hdr := &tar.Header{
		Typeflag: tar.TypeXGlobalHeader,
		Name:     "pax_global_header",
		PAXRecords: map[string]string{
			"comment": sha.String(),
		},
	}

	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}

	for _, e := range entries {
		o, err := c.GetObject(e.Sha1)
		if err != nil {
			return err
		}
		if obj, ok := o.(GitBlobObject); ok {
			hdr := &tar.Header{}
			hdr.Name = opts.BasePrefix + e.PathName.String()
			hdr.Size = int64(obj.GetSize())
			hdr.ModTime = mtime
			hdr.Uid = 0
			hdr.Gid = 0
			hdr.Uname = "root"
			hdr.Gname = "root"

			// TODO: Mask the mode. by default the mask is 0002 (turn off write bit)
			// but can be changed using tar.umask config.
			switch e.Mode {
			case ModeBlob:
				hdr.Mode = 0644
			case ModeExec:
				hdr.Mode = 0755
			default:
				hdr.Mode = 0644
			}

			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}
			if _, err := tw.Write(o.GetContent()); err != nil {
				return err
			}
		}
	}
	return nil
}

func createZipArchive(c *Client, opts ArchiveOptions, sha Sha1, entries []*IndexEntry) error {
	fileOutput := os.Stdout

	// If the output file is not empty use it instead of stdout
	if opts.OutputFile != "" {
		if file, err := os.Create(opts.OutputFile); err == nil {
			fileOutput = file
			defer file.Close()
		} else {
			return err
		}
	}

	mtime := time.Now()

	// If the sha is a tree we set the file modification time to the current time
	// otherwise we must use the commit time.
	if sha.Type(c) != "tree" {
		if t, err := CommitID(sha).GetDate(c); err == nil {
			mtime = t
		}
	}

	zw := zip.NewWriter(fileOutput)
	defer zw.Close()

	// set the compression level
	zw.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, opts.CompressionLevel)
	})

	// Set the commit sha as the zip comment.
	zw.SetComment(sha.String())

	for _, e := range entries {
		o, err := c.GetObject(e.Sha1)
		if err != nil {
			return err
		}
		if obj, ok := o.(GitBlobObject); ok {
			hdr := &zip.FileHeader{
				Name:     opts.BasePrefix + e.PathName.String(),
				Modified: mtime,
				Method:   zip.Deflate,
			}
			f, err := zw.CreateHeader(hdr)

			if err != nil {
				return nil
			}

			if _, err := f.Write(obj.GetContent()); err != nil {
				return err
			}
		}
	}

	return nil
}

// Return the list of supported archive file format
func ArchiveFormatList() []string {
	formats := make([]string, 0, len(supportedArchiveFormats))
	for key := range supportedArchiveFormats {
		formats = append(formats, key)
	}
	return formats
}

func Archive(c *Client, opts ArchiveOptions, arg string) error {
	format := archiveTar

	//
	treeish, err := RevParseTreeish(c, &RevParseOptions{}, arg)
	if err != nil {
		return err
	}

	// find the format output desired
	if opts.ArchiveFormat != "" {
		format, err = findOutputFileFormat(opts.ArchiveFormat)
		if err != nil {
			return fmt.Errorf("Unknow archive format '%s'", opts.ArchiveFormat)
		}
	} else if opts.OutputFile != "" {
		// if the output file is not empty try to find
		// the archive format from the file extension.
		format, _ = findOutputFileFormat(opts.OutputFile)
	}

	// commit hash
	var sha1 Sha1

	// if the input it's a tag we must resolve it to a commit
	if h, ok := treeish.(Ref); ok {
		refspec := RefSpec(h.Name)
		if c, err := refspec.CommitID(c); err != nil {
			return err
		} else {
			treeish = c
		}
	}

	if s, ok := treeish.(CommitID); ok {
		sha1 = Sha1(s)
	} else {
		return fmt.Errorf("Can't convert treeish to commit id")
	}

	lstree, err := LsTree(c, LsTreeOptions{Recurse: true}, treeish, nil)
	if err != nil {
		return err
	}

	switch format {
	case archiveTar:
		return createTarArchive(c, opts, false, sha1, lstree)
	case archiveTarGzip:
		return createTarArchive(c, opts, true, sha1, lstree)
	case archiveZip:
		return createZipArchive(c, opts, sha1, lstree)
	}
	return nil
}
