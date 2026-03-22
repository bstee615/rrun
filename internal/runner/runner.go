package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"rrun/internal/config"
)

// State is written as .rrun in the synced directory root on the remote.
type State struct {
	SourceMachine string    `json:"source_machine"`
	SourcePath    string    `json:"source_path"`
	LastSync      time.Time `json:"last_sync"`
	LastCommand   string    `json:"last_command,omitempty"`
}

// GitRoot returns the absolute path to the root of the current git repository.
func GitRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// RemoteDir computes the remote path for a given local directory. If the
// remote has a path_map configured and the local path matches the prefix,
// the prefix is replaced. Otherwise the local path is mirrored exactly.
func RemoteDir(localDir string, r config.Remote) string {
	if r.PathMap.Local != "" && r.PathMap.Remote != "" {
		if rel, ok := strings.CutPrefix(localDir, r.PathMap.Local); ok {
			return r.PathMap.Remote + rel
		}
	}
	return localDir
}

// Sync rsyncs git-tracked files from localDir to remoteDir on the remote host.
// .rrun is excluded from deletion so state persists across syncs.
func Sync(remote config.Remote, localDir, remoteDir string, verbose bool) error {
	tracked, err := exec.Command("git", "-C", localDir, "ls-files", "-z").Output()
	if err != nil {
		return fmt.Errorf("git ls-files: %w", err)
	}

	// Ensure the remote directory exists before rsyncing into it.
	if err := sshRun(remote.Host, "mkdir -p "+shellescape(remoteDir)); err != nil {
		return fmt.Errorf("remote mkdir: %w", err)
	}

	args := []string{
		"-az",
		"--files-from=-", "--from0",
	}
	if verbose {
		args = append(args, "-v")
	}
	args = append(args, localDir+"/", remote.Host+":"+remoteDir+"/")

	rsync := exec.Command("rsync", args...)
	rsync.Stdin = strings.NewReader(string(tracked))
	rsync.Stdout = os.Stdout
	rsync.Stderr = os.Stderr
	return rsync.Run()
}

// WriteState writes a .rrun metadata file to remoteDir on the remote.
func WriteState(remote config.Remote, localDir, remoteDir, lastCmd string) error {
	hostname, _ := os.Hostname()
	state := State{
		SourceMachine: hostname,
		SourcePath:    localDir,
		LastSync:      time.Now().UTC().Truncate(time.Second),
		LastCommand:   lastCmd,
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	cmd := exec.Command("ssh", remote.Host,
		fmt.Sprintf("cat > %s", shellescape(remoteDir+"/.rrun")))
	cmd.Stdin = strings.NewReader(string(data)+"\n")
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Run executes args on the remote inside remoteDir, with a PTY so output
// streams live (progress bars, colours, etc. all work).
func Run(remote config.Remote, remoteDir string, args []string) error {
	escaped := make([]string, len(args))
	for i, a := range args {
		escaped[i] = shellescape(a)
	}
	remoteCmd := fmt.Sprintf("cd %s && %s", shellescape(remoteDir), strings.Join(escaped, " "))
	ssh := exec.Command("ssh", "-t", remote.Host, remoteCmd)
	ssh.Stdin = os.Stdin
	ssh.Stdout = os.Stdout
	ssh.Stderr = os.Stderr
	return ssh.Run()
}

func sshRun(host, cmd string) error {
	c := exec.Command("ssh", host, cmd)
	c.Stderr = os.Stderr
	return c.Run()
}

// shellescape wraps s in single quotes, escaping any embedded single quotes.
func shellescape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
