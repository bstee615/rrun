package cmd

import (
	"fmt"

	"rrun/internal/config"
)

// resolveRemote returns the Remote to use, preferring the --remote flag,
// then the configured default, erroring if neither is set.
func resolveRemote() (config.Remote, string, error) {
	cfg, err := config.Load()
	if err != nil {
		return config.Remote{}, "", fmt.Errorf("loading config: %w", err)
	}

	name := flagRemote
	if name == "" {
		name = cfg.DefaultRemote
	}
	if name == "" {
		return config.Remote{}, "", fmt.Errorf("no remote specified; add one with: rrun remote add <name> <host>")
	}

	r, ok := cfg.Remotes[name]
	if !ok {
		return config.Remote{}, "", fmt.Errorf("remote %q not found in config", name)
	}
	return r, name, nil
}
