package git

// Returns true if a reflog exists for refname r under client.
func ReflogExists(c *Client, r Refname) bool {
	return c.GitDir.File(File(r)).Exists()
}
