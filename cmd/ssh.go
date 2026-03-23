package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"

	"github.com/bstee615/rrun/internal/runner"
)

var sshCmd = &cobra.Command{
	Use:          "ssh",
	Short:        "Open an interactive SSH session on the remote, cd'd to the current directory",
	SilenceUsage: true,
	RunE:         runSSH,
}

func init() {
	rootCmd.AddCommand(sshCmd)
}

func runSSH(_ *cobra.Command, _ []string) error {
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

	cleanHost, port := runner.ParseHostPort(remote.Host)

	// cd to the remote work dir then hand off to the user's login shell.
	remoteCmd := fmt.Sprintf("cd %s && exec bash -l", runner.Shellescape(remoteWorkDir))

	sshArgs := []string{"-t"}
	if port != 0 {
		sshArgs = append(sshArgs, "-p", strconv.Itoa(port))
	}
	sshArgs = append(sshArgs, cleanHost, remoteCmd)

	log.Info("Connecting", "host", remote.Host, "dir", remoteWorkDir)

	cmd := exec.Command("ssh", sshArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
