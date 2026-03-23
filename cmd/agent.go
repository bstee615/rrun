package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"

	"github.com/bstee615/rrun/internal/runner"
)

var flagRepo bool

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Install AI agent skills for using rrun",
	Long: `Manage AI coding agent skills so that Claude Code, GitHub Copilot CLI,
and VS Code Copilot know how to sync and run commands with rrun.

Run "rrun agent install" after installing rrun to teach your agents.`,
}

var agentInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install rrun skills for AI coding agents",
	Long: `Install slash commands and instructions so AI agents know how to use rrun.

By default, installs global slash commands to ~/.claude/commands/.
These are picked up by Claude Code and GitHub Copilot CLI.

With --repo, additionally installs per-repo instructions to
.github/instructions/rrun.instructions.md in the current git repository.
These are picked up by GitHub Copilot in VS Code and all other agents.`,
	SilenceUsage: true,
	RunE:         agentInstall,
}

var agentUninstallCmd = &cobra.Command{
	Use:          "uninstall",
	Short:        "Remove rrun agent skills",
	SilenceUsage: true,
	RunE:         agentUninstall,
}

var agentStatusCmd = &cobra.Command{
	Use:          "status",
	Short:        "Show which rrun agent skills are installed",
	SilenceUsage: true,
	RunE:         agentStatus,
}

func init() {
	agentInstallCmd.Flags().BoolVar(&flagRepo, "repo", false, "also install per-repo instructions for VS Code Copilot")
	agentUninstallCmd.Flags().BoolVar(&flagRepo, "repo", false, "also remove per-repo instructions")

	agentCmd.AddCommand(agentInstallCmd, agentUninstallCmd, agentStatusCmd)
	rootCmd.AddCommand(agentCmd)
}

// ---------------------------------------------------------------------------
// Skill content
// ---------------------------------------------------------------------------

const skillRunMD = `Sync the current project to the remote and run a command there.

Usage: /rrun-run <command and args>

Steps:
1. If no command was given after /rrun-run, ask the user what command to run on the remote.
2. Run ` + "`rrun run <command>`" + ` from the git root directory.
3. Stream the output. The command runs on the remote with a live PTY so output appears as it happens.
4. Report the exit code when it finishes.
5. If the command fails, show the exit code and any error output. Do not attempt to re-run automatically.

Common patterns:
  /rrun-run python train.py --epochs 10
  /rrun-run make test
  /rrun-run bash -c 'nvidia-smi && python bench.py'
  /rrun-run --remote workstation pytest tests/
`

const skillSyncMD = `Sync the current git project to the configured remote machine using rrun.

Steps:
1. Check rrun is configured by running ` + "`rrun remote list`" + `. If no remotes exist, tell the user to run ` + "`rrun remote add <name> <host>`" + ` first.
2. Run ` + "`rrun sync`" + ` from the git root directory.
3. If the sync succeeds, report what remote it synced to.
4. If the sync fails, diagnose the error:
   - SSH connection errors → check host reachability and key setup
   - Permission errors → check remote directory ownership
   - Missing rsync/ssh → prompt to install
   Show the relevant error and suggest a fix.
`

const repoInstructionsMD = `# rrun — Remote Sync & Run

rrun syncs git-tracked files to a remote machine and runs commands there.
Use it when the user needs to build, test, or run code on a remote server (e.g. a GPU machine).

## Quick Reference

- ` + "`rrun remote list`" + ` — show configured remotes
- ` + "`rrun sync`" + ` — sync current project to the default remote
- ` + "`rrun run <command>`" + ` — sync and run a command on the remote
- ` + "`rrun run --remote <name> <command>`" + ` — use a specific remote
- ` + "`rrun ssh`" + ` — open an interactive SSH session on the remote
- ` + "`rrun dir`" + ` — print the remote path for the current directory

## Useful flags

| Flag | Effect |
|---|---|
| ` + "`--remote <name>`" + ` | Use a specific remote instead of the default |
| ` + "`--delete`" + ` | Remove remote files no longer tracked by git (clean sync) |
| ` + "`--verbose`" + ` | Show each file rsync transfers |
| ` + "`--quiet`" + ` | Suppress info/warn messages |

## When to use rrun

- User asks to run something on a remote/GPU machine
- User mentions their remote or a named machine configured with rrun
- User needs to sync code before running remote commands

## Error handling

- If no remotes are configured: suggest ` + "`rrun remote add <name> <host>`" + `
- SSH connection errors: check host reachability and SSH key setup
- Permission errors: check remote directory ownership
- Use ` + "`--delete`" + ` after renaming or deleting files to clean the remote
`

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type skillFile struct {
	path    string
	content string
	label   string
}

func globalSkillFiles() ([]skillFile, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}
	dir := filepath.Join(home, ".claude", "commands")
	return []skillFile{
		{filepath.Join(dir, "rrun-run.md"), skillRunMD, "slash command /rrun-run"},
		{filepath.Join(dir, "rrun-sync.md"), skillSyncMD, "slash command /rrun-sync"},
	}, nil
}

func repoSkillFiles() ([]skillFile, error) {
	root, err := runner.GitRoot()
	if err != nil {
		return nil, fmt.Errorf("not in a git repository (needed for --repo): %w", err)
	}
	dir := filepath.Join(root, ".github", "instructions")
	return []skillFile{
		{filepath.Join(dir, "rrun.instructions.md"), repoInstructionsMD, "repo instructions"},
	}, nil
}

func writeSkill(sf skillFile) error {
	dir := filepath.Dir(sf.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}
	return os.WriteFile(sf.path, []byte(sf.content), 0o644)
}

// ---------------------------------------------------------------------------
// Commands
// ---------------------------------------------------------------------------

func agentInstall(_ *cobra.Command, _ []string) error {
	files, err := globalSkillFiles()
	if err != nil {
		return err
	}

	if flagRepo {
		repo, err := repoSkillFiles()
		if err != nil {
			return err
		}
		files = append(files, repo...)
	}

	for _, sf := range files {
		if err := writeSkill(sf); err != nil {
			return fmt.Errorf("writing %s: %w", sf.label, err)
		}
		log.Info("Installed", "skill", sf.label, "path", sf.path)
	}

	fmt.Println()
	fmt.Println("Agents with rrun skills:")
	fmt.Println("  ✓ Claude Code        (slash commands)")
	fmt.Println("  ✓ GitHub Copilot CLI (slash commands)")
	if flagRepo {
		fmt.Println("  ✓ VS Code Copilot    (repo instructions)")
	} else {
		fmt.Println("  · VS Code Copilot    (use --repo to install per-repo instructions)")
	}
	return nil
}

func agentUninstall(_ *cobra.Command, _ []string) error {
	files, err := globalSkillFiles()
	if err != nil {
		return err
	}

	if flagRepo {
		repo, err := repoSkillFiles()
		if err != nil {
			return err
		}
		files = append(files, repo...)
	}

	for _, sf := range files {
		if err := os.Remove(sf.path); err != nil {
			if os.IsNotExist(err) {
				log.Info("Already removed", "skill", sf.label)
				continue
			}
			return fmt.Errorf("removing %s: %w", sf.label, err)
		}
		log.Info("Removed", "skill", sf.label, "path", sf.path)
	}
	return nil
}

func agentStatus(_ *cobra.Command, _ []string) error {
	files, err := globalSkillFiles()
	if err != nil {
		return err
	}

	// Try to get repo files, but don't fail if not in a git repo.
	repo, repoErr := repoSkillFiles()
	if repoErr == nil {
		files = append(files, repo...)
	}

	installed := 0
	for _, sf := range files {
		if _, err := os.Stat(sf.path); err == nil {
			fmt.Printf("  ✓ %s\n    %s\n", sf.label, sf.path)
			installed++
		} else {
			fmt.Printf("  · %s  (not installed)\n", sf.label)
		}
	}

	if installed == 0 {
		fmt.Println("\nNo skills installed. Run: rrun agent install")
	}
	return nil
}
