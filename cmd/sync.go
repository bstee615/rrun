package cmd

import (
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"rrun/internal/runner"
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
	remote, remoteName, err := resolveRemote()
	if err != nil {
		return err
	}

	localDir, err := runner.GitRoot()
	if err != nil {
		return fmt.Errorf("not in a git repository: %w", err)
	}

	remoteDir := runner.RemoteDir(localDir, remote)

	log.Info("Syncing", "remote", remoteName, "host", remote.Host, "path", remoteDir)
	if err := runner.Sync(remote, localDir, remoteDir, flagVerbose); err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	if err := runner.WriteState(remote, localDir, remoteDir, ""); err != nil {
		log.Warn("Failed to write .rrun", "err", err)
	}

	log.Info("Sync complete")
	return nil
}
