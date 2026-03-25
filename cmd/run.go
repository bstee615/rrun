package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"

	"github.com/bstee615/rrun/internal/config"
	"github.com/bstee615/rrun/internal/runner"
)

var runCmd = &cobra.Command{
	Use:   "run [flags] [command...]",
	Short: "Sync files to the remote then run a command there",
	Long: `Syncs git-tracked files to the remote, updates the .rrun state file,
then executes the given command on the remote with live-streamed output.

Examples:
  rrun run python train.py --epochs 10
  rrun run --remote lambda make test
  rrun run --delete bash -c 'nvidia-smi && python bench.py'`,
	SilenceUsage: true,
	RunE:         runRun,
}

func init() {
	// Stop cobra from consuming flags intended for the remote command.
	runCmd.Flags().SetInterspersed(false)
	rootCmd.AddCommand(runCmd)
}

func runRun(_ *cobra.Command, args []string) error {
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
	remoteWorkDir, err := runner.RemoteWorkDir(localDir, remoteDir)
	if err != nil {
		return err
	}

	cmdStr := strings.Join(args, " ")

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
		if err := conn.WriteState(localDir, remoteDir, cmdStr); err != nil {
			log.Warn("Failed to write .rrun", "err", err)
		}
	}

	if len(args) == 0 {
		log.Info("Sync complete (no command given)")
		return nil
	}

	log.Info("Running", "cmd", cmdStr)
	return conn.Run(remoteWorkDir, args)
}
