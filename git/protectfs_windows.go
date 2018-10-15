package git

func protectHFS(c *Client) bool {
	return c.GetConfig("core.protectHFS") == "true"
}

// protectNTFS defaults to true on windows
func protectNTFS(c *Client) bool {
	return c.GetConfig("core.protectNTFS") != "false"
}
