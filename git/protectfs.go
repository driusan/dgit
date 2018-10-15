// +build !windows
// +build !darwin

package git

func protectHFS(c *Client) bool {
	return c.GetConfig("core.protectHFS") == "true"
}

func protectNTFS(c *Client) bool {
	return c.GetConfig("core.protectNTFS") == "true"
}
