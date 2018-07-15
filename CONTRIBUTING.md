# Contributing!

If you'd like to contribute, I'd love to have your help, but it might help to
know the code/package layout to get started.

There's 3 subpackages (and a main.go) to this repo. 
1. `main.go` parses the main git options, and initializes the *Client which is
   used throughout the code to manipulate the git repo, and calls the `cmd`
   package.
2. `github.com/driusan/dgit/git` is a package which contains two things:
   abstract types that don't directly map to git command line usage (such as
   `type Client struct{...}` or `type File string`), and functions which implement
   the git command line and return values in terms of Go types. This package
   should be safe to use in other programs that want to interrogate/manipulate
   git repos in a Go-ish way without execing 'git', but isn't stable yet.
3. `github.com/driusan/dgit/cmd` is a package which parses the os.Args, converts
   it to `package git` types, invokes the `git` package, and prints the result. It's
   the glue between the command line and the Go.
4. zlib is a hacked up version of the standard `compress/zlib` which tries to
   make it possible to decompress pack files when the Reader reads too much of
   the file. If this doesn't make sense to you, ignore it. It doesn't matter
   unless you're trying to index a pack file (or decompress one without the index.)

The distinction between `git` and `cmd` packages isn't as clean as it should be.
(It all started off in one package, then `type Client` was moved to `git`,
all the commands were moved to `cmd`, and then interdependent stuff was slowly
moved around to the right package until it compiled. The refactoring of old code
isn't as complete as it should be, but new code should be written with this
distinction in mind.)

I generally try to keep the file status.txt in the root of this repo up to date
with the status of subcommands. If you'd like to contribute, a good place to
start would be looking at that, taking your favourite command, and either
adding missing options, or adding the command itself. If something claims to
be implemented in the status.txt, you can also compare it to the official `git`
client on your system and file bugs.

Plumbing commands should be a higher priority than porcelain commands, and
eventually the porcelain commands that exist should be re-written in terms of
the underlying plumbing to be more robust.
