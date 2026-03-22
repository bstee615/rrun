package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"rrun/internal/runner"
)

var runCmd = &cobra.Command{
	Use:   "run [flags] [command...]",
	Short: "Sync files to the remote then run a command there",
	Long: `Syncs git-tracked files to the remote, updates the .rrun state file,
then executes the given command on the remote with live-streamed output.

Examples:
  rrun run python train.py --epochs 10
  rrun run --remote lambda make test
  rrun run bash -c 'nvidia-smi && python bench.py'`,
	SilenceUsage: true,
	RunE:         runRun,
}

func init() {
	// Stop flag parsing at the first non-flag argument so that flags intended
	// for the remote command (e.g. --epochs) are not consumed by rrun.
	runCmd.Flags().SetInterspersed(false)
	rootCmd.AddCommand(runCmd)
}

func runRun(_ *cobra.Command, args []string) error {
	remote, remoteName, err := resolveRemote()
	if err != nil {
		return err
	}

	localDir, err := runner.GitRoot()
	if err != nil {
		return fmt.Errorf("not in a git repository: %w", err)
	}

	remoteDir := runner.RemoteDir(localDir, remote)
	cmdStr := strings.Join(args, " ")

	log.Info("Syncing", "remote", remoteName, "host", remote.Host, "path", remoteDir)
	if err := runner.Sync(remote, localDir, remoteDir, flagVerbose); err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	if err := runner.WriteState(remote, localDir, remoteDir, cmdStr); err != nil {
		log.Warn("Failed to write .rrun", "err", err)
	}

	if len(args) == 0 {
		log.Info("Sync complete (no command given)")
		return nil
	}

	log.Info("Running", "cmd", cmdStr)
	return runner.Run(remote, remoteDir, args)
}
