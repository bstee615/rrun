package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/bstee615/rrun/internal/runner"
)

var dirCmd = &cobra.Command{
	Use:          "dir",
	Short:        "Print the remote path equivalent of the current directory",
	SilenceUsage: true,
	RunE:         runDir,
}

func init() {
	rootCmd.AddCommand(dirCmd)
}

func runDir(_ *cobra.Command, _ []string) error {
	remote, _, err := resolveRemote()
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
	fmt.Println(remoteWorkDir)
	return nil
}
