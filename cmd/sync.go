package cmd

import (
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"

	"github.com/bstee615/rrun/internal/config"
	"github.com/bstee615/rrun/internal/runner"
)

var syncCmd = &cobra.Command{
	Use:          "sync",
	Short:        "Sync git-tracked files to the remote",
	SilenceUsage: true,
	RunE:         runSync,
}

func init() {
	rootCmd.AddCommand(syncCmd)
}

func runSync(_ *cobra.Command, _ []string) error {
	if err := runner.CheckDeps(); err != nil {
		return err
	}

	remote, remoteName, err := resolveRemote()
	if err != nil {
		return err
	}

	localDir, err := runner.GitRoot()
	if err != nil {
		return err
	}

	remoteDir := runner.RemoteDir(localDir, remote)

	log.Info("Syncing", "remote", remoteName, "host", remote.Host, "path", remoteDir)

	conn, err := runner.Dial(remote)
	if err != nil {
		return err
	}
	defer conn.Close()

	cfg, _ := config.Load()
	var retryCfg config.RetryConfig
	var warnMB int
	if cfg != nil {
		retryCfg = cfg.Retry
		warnMB = cfg.LargeTransferWarnMB
	}

	if err := conn.Sync(localDir, remoteDir, syncArgs(), retryCfg, warnMB); err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	noState := flagNoState || (cfg != nil && cfg.NoState)
	if !noState {
		if err := conn.WriteState(localDir, remoteDir, ""); err != nil {
			log.Warn("Failed to write .rrun", "err", err)
		}
	}

	log.Info("Sync complete")
	return nil
}
