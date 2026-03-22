package cmd

import (
	"os"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
)

var (
	flagRemote  string
	flagVerbose bool
)

var rootCmd = &cobra.Command{
	Use:   "rrun",
	Short: "Sync git-tracked files to a remote machine and run commands there",
	Long: `
  _,-. ,-. ,-,
 (ô )────────────  rrun
  \_)_)            sync & run on remote machines

Syncs files tracked by git to a remote machine and runs commands there
with live-streamed output. Works with any git repo, no project config needed.

  rrun remote add workstation gpu-box
  rrun sync
  rrun run python train.py --epochs 10`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&flagRemote, "remote", "r", "", "named remote to use (overrides configured default)")
	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "verbose rsync output")
	log.SetLevel(log.InfoLevel)
}
