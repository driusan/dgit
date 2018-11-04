package git

import (
	"archive/tar"
	"archive/zip"
	"compress/flate"
	"compress/gzip"
	"io"
	"os"
	"time"
)

type ArchiveOptions struct {
	Verbose            bool
	List               bool
	WorktreeAttributes bool
	Format             ArchiveFormat
	BasePrefix         string
	OutputFile         *os.File
	CompressionLevel   int
}

type ArchiveFormat int

const (
	ArchiveTar = ArchiveFormat(iota)
	ArchiveTarGzip
	ArchiveZip
)

var supportedArchiveFormats = map[string]ArchiveFormat{
	"tar":    ArchiveTar,
	"tgz":    ArchiveTarGzip,
	"tar.gz": ArchiveTarGzip,
	"zip":    ArchiveZip,
}

func createTarArchive(c *Client, opts ArchiveOptions, tgz bool, sha Sha1, mtime time.Time, entries []*IndexEntry) error {
	var fileOutput io.Writer = os.Stdout

	// If the output file is set use it instead of stdout
	if opts.OutputFile != nil {
		fileOutput = opts.OutputFile
	}

	// gzip compression is enabled
	if tgz {
		gw := gzip.NewWriter(fileOutput)
		fileOutput = gw
		defer gw.Close()
	}

	// Create the tar writer
	tw := tar.NewWriter(fileOutput)
	defer tw.Close()

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

func createZipArchive(c *Client, opts ArchiveOptions, sha Sha1, mtime time.Time, entries []*IndexEntry) error {
	fileOutput := os.Stdout

	// If the output file is set use it instead of stdout
	if opts.OutputFile != nil {
		fileOutput = opts.OutputFile
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
func ArchiveFormatList() map[string]ArchiveFormat {
	return supportedArchiveFormats
}

func Archive(c *Client, opts ArchiveOptions, tree Treeish) error {
	// commit hash
	var sha1 Sha1

	mtime := time.Now()

	commitish, ok := tree.(Commitish)
	if ok {
		cid, err := commitish.CommitID(c)
		if err != nil {
			return err
		}
		sha1 = Sha1(cid)

		// If the sha is a tree we set the file modification time to the current time
		// otherwise we must use the commit time.
		if t, err := cid.GetDate(c); err == nil {
			mtime = t
		}
	}

	lstree, err := LsTree(c, LsTreeOptions{Recurse: true}, tree, nil)
	if err != nil {
		return err
	}

	switch opts.Format {
	case ArchiveTar:
		return createTarArchive(c, opts, false, sha1, mtime, lstree)
	case ArchiveTarGzip:
		return createTarArchive(c, opts, true, sha1, mtime, lstree)
	case ArchiveZip:
		return createZipArchive(c, opts, sha1, mtime, lstree)
	}
	return nil
}
