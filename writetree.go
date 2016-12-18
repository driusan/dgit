package main

// WriteTree implements the git write-tree command on the Git repository
// pointed to by c.
func WriteTree(c *Client) string {
	idx, _ := c.GitDir.ReadIndex()
	sha1 := idx.WriteTree(c)
	return sha1
}
