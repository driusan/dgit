package git

// protectHFS defaults to true on Mac
func protectHFS(c *Client) bool {
	return config.GetConfig("core.protectHFS") != "false"
}

func protectNTFS(c *Client) bool {
	return config.GetConfig("core.protectNTFS") == "true"
}
