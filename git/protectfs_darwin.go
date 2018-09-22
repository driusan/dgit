package git

// protectHFS defaults to true on Mac
func protectHFS(c *Client) bool {
	config, err := LoadLocalConfig(c)
	if err == nil {
		if cfg, _ := config.GetConfig("core.protectHFS"); cfg != "" {
			return cfg == "true"
		}
	}
	config, err = LoadGlobalConfig()
	if err == nil {
		if cfg, _ := config.GetConfig("core.protectHFS"); cfg != "" {
			return cfg == "true"
		}
	}
	return true

}

func protectNTFS(c *Client) bool {
	config, err := LoadLocalConfig(c)
	if err == nil {
		if cfg, _ := config.GetConfig("core.protectNTFS"); cfg != "" {
			return cfg == "true"
		}
	}
	config, err = LoadGlobalConfig()
	if err == nil {
		if cfg, _ := config.GetConfig("core.protectNTFS"); cfg != "" {
			return cfg == "true"
		}
	}
	return false
}
