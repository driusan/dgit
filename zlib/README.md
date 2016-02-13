This directory contains a fork of the compress/zlib algorithm from
Go.

It's modified so that NewReader returns a ZLibReader instead of an
io.Reader, and a function is added to ZLibReader to expose the 
digest of the compressed block after reading it.

This allows you to look back for the address of the block after
reading (assuming that the underlying io.Reader is an io.ReadSeeker.)
to find the end of the compressed block, even though the zlib library
reads from the io.Reader greedily and might (read: does) read more than
it needs to from the io.Reader given to it.
