package cmd

import (
	"fmt"

	"github.com/bstee615/rrun/internal/config"
	"github.com/bstee615/rrun/internal/runner"
)

var flagGlobal bool

// loadRemoteConfig returns the remotes map, a pointer to the default-remote
// field, and a save function for either the project config (default) or the
// global config (when --global is set).
//
// The caller modifies remotes and *defaultRemote directly, then calls save().
func loadRemoteConfig() (remotes map[string]config.Remote, defaultRemote *string, save func() error, err error) {
	if flagGlobal {
		cfg, err := config.Load()
		if err != nil {
			return nil, nil, nil, err
		}
		return cfg.Remotes, &cfg.DefaultRemote, func() error { return config.Save(cfg) }, nil
	}

	gitRoot, err := runner.GitRoot()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("not in a git repository (use --global for the global config): %w", err)
	}

	proj, err := config.LoadProject(gitRoot)
	if err != nil {
		return nil, nil, nil, err
	}
	if proj == nil {
		proj = &config.ProjectConfig{Remotes: make(map[string]config.Remote)}
	}
	if proj.Remotes == nil {
		proj.Remotes = make(map[string]config.Remote)
	}
	return proj.Remotes, &proj.DefaultRemote, func() error { return config.SaveProject(gitRoot, proj) }, nil
}

// resolveRemote returns the Remote to use. Priority order:
//  1. --remote flag
//  2. default_remote from .rrun.yaml in the git root
//  3. default_remote from the global config
func resolveRemote() (config.Remote, string, error) {
	cfg, err := config.Load()
	if err != nil {
		return config.Remote{}, "", fmt.Errorf("loading config: %w", err)
	}

	// Load project config and merge: project remotes shadow global ones,
	// and project default_remote takes precedence over the global default.
	var projDefaultRemote string
	if gitRoot, err := runner.GitRoot(); err == nil {
		if proj, err := config.LoadProject(gitRoot); err == nil && proj != nil {
			projDefaultRemote = proj.DefaultRemote
			for k, v := range proj.Remotes {
				cfg.Remotes[k] = v
			}
		}
	}

	name := flagRemote
	if name == "" {
		name = projDefaultRemote
	}
	if name == "" {
		name = cfg.DefaultRemote
	}
	if name == "" {
		return config.Remote{}, "", fmt.Errorf("no remote specified; add one with: rrun remote add <name> <host>")
	}

	r, ok := cfg.Remotes[name]
	if !ok {
		return config.Remote{}, "", fmt.Errorf("remote %q not found; list remotes with: rrun remote list", name)
	}
	return r, name, nil
}

// syncArgs builds a SyncOptions from the current global flags.
func syncArgs() runner.SyncOptions {
	return runner.SyncOptions{
		Verbose: flagVerbose,
		Delete:  flagDelete,
	}
}
