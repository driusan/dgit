package git

// Denotes a username/password combo to be used to
// authenticate (generally for an HTTP connection
// while pushing.)
type userPasswd struct {
	user, password string
}
