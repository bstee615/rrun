package cmd

import (
	"os"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"

	"rrun/internal/config"
	"rrun/internal/logging"
)

// version is injected at build time via -ldflags "-X rrun/cmd.version=x.y.z"
var version = "dev"

var (
	flagRemote  string
	flagVerbose bool
	flagQuiet   bool
	flagNoState bool
	flagDelete  bool
)

var rootCmd = &cobra.Command{
	Use:     "rrun",
	Version: version,
	Short:   "Sync git-tracked files to a remote machine and run commands there",
	Long: `
  _,-. ,-. ,-,
 (ô )────────────  rrun
  \_)_)            sync & run on remote machines

Syncs files tracked by git to a remote machine and runs commands there
with live-streamed output. Works with any git repo, no project config needed.

  rrun remote add workstation gpu-box
  rrun sync
  rrun run python train.py --epochs 10`,

	// Init logging before any subcommand runs.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := config.Load()

		// Effective quiet: flag OR config file.
		quiet := flagQuiet
		if cfg != nil && cfg.Quiet {
			quiet = true
		}
		if quiet {
			log.SetLevel(log.ErrorLevel)
		}

		// Init file logger.
		logPath := ""
		if cfg != nil {
			logPath = cfg.LogPath
		}
		if err := logging.Init(logPath); err != nil {
			// Non-fatal: file logging is best-effort.
			log.Debug("Could not init file logger", "err", err)
		}

		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&flagRemote, "remote", "r", "", "named remote to use (overrides configured default)")
	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "verbose rsync output (lists each file)")
	rootCmd.PersistentFlags().BoolVarP(&flagQuiet, "quiet", "q", false, "suppress info/warn messages (errors still shown)")
	rootCmd.PersistentFlags().BoolVar(&flagNoState, "no-state", false, "do not write .rrun metadata file on the remote")
	rootCmd.PersistentFlags().BoolVar(&flagDelete, "delete", false, "remove files on the remote that are no longer tracked by git")
}
