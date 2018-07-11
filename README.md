![travis ci](https://api.travis-ci.org/driusan/dgit.svg?branch=master)

This repo contains a pure Go implementation of the command
line git client that I've been working on so that I can
try doing some of my Go development on Plan 9.

It's primary purpose is to enable users of operating systems
where Go is supported but the canonical git implementation
isn't (ie. Plan 9) to use git.

IT IS NOWHERE NEAR READY.

The main goal is to enable `go get` to work by doing a real
checkout and without any hacks such as downloading a .zip from
GitHub and pretending it's a checkout. (This should be done, 
and any breakages with `go get` if you rename the binary from
`go-git` to `git` and put it on your path should be reported
as an issue.)

The secondary goal is to enable just enough of the git
command line to allow simple development (ie. simple usages of
git add/commit/push/status/diff/log.)

The third, stretch goal, is to have a complete command-line
compatible implementation of git that can be used as a drop-in
replacement for git.
