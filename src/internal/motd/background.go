package motd

// needsRelativeBackgrounds reports whether any configured background uses a
// relative or blend value that must be resolved against the terminal color.
func needsRelativeBackgrounds(cfg Config) bool {
	return cfg.BackgroundRelative != 0 || cfg.BackgroundBlendSet ||
		cfg.WindowBackgroundRelative != 0 || cfg.WindowBackgroundBlendSet ||
		cfg.Description.BackgroundRelative != 0 ||
		cfg.Header.BackgroundRelative != 0
}

// needsTerminalBackground reports whether rendering needs the terminal
// background color. This is used by Preflight to decide whether to probe.
func needsTerminalBackground(cfg Config) bool {
	// Any window background needs the terminal color to fade the margins.
	if cfg.WindowBackground != "" {
		return true
	}
	// Card/window/header relative values (or description with no card color)
	// need a query.
	if cfg.BackgroundRelative != 0 || cfg.BackgroundBlendSet ||
		cfg.WindowBackgroundRelative != 0 || cfg.WindowBackgroundBlendSet ||
		cfg.Header.BackgroundRelative != 0 {
		return true
	}
	// A transparent card with codeblocks shades the codeblock surface from
	// the terminal color, so the query is still required.
	if cfg.Background == "" && len(cfg.Recipes) > 0 {
		return true
	}
	return cfg.Description.BackgroundRelative != 0 && cfg.Background == ""
}
