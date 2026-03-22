package runner

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/charmbracelet/log"

	"rrun/internal/config"
	"rrun/internal/logging"
)

// State is written as .rrun in the synced directory root on the remote.
type State struct {
	SourceMachine string    `json:"source_machine"`
	SourcePath    string    `json:"source_path"`
	LastSync      time.Time `json:"last_sync"`
	LastCommand   string    `json:"last_command,omitempty"`
}

// SyncOptions configures a sync operation.
type SyncOptions struct {
	Verbose bool
	Delete  bool // pass --delete to rsync (removes files on remote not in git)
}

// CheckDeps verifies that the required external binaries are in PATH.
func CheckDeps() error {
	for _, bin := range []string{"rsync", "ssh"} {
		if _, err := exec.LookPath(bin); err != nil {
			return fmt.Errorf("%s not found in PATH — install it first (e.g. sudo pacman -S %s)", bin, bin)
		}
	}
	return nil
}

// GitRoot returns the absolute path to the root of the current git repository.
func GitRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("not inside a git repository")
	}
	return strings.TrimSpace(string(out)), nil
}

// RemoteDir computes the remote path for a given local directory, applying
// path_map prefix substitution if configured.
func RemoteDir(localDir string, r config.Remote) string {
	if r.PathMap.Local != "" && r.PathMap.Remote != "" {
		if rel, ok := strings.CutPrefix(localDir, r.PathMap.Local); ok {
			return r.PathMap.Remote + rel
		}
	}
	return localDir
}

// RemoteWorkDir returns the remote equivalent of the current working directory.
// It computes the relative path from localGitRoot to cwd, then appends it to
// the remote git root. This mirrors the user's position within the repo on the remote.
func RemoteWorkDir(localGitRoot, remoteGitRoot string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return remoteGitRoot, err
	}
	rel, err := filepath.Rel(localGitRoot, cwd)
	if err != nil || rel == "." {
		return remoteGitRoot, nil
	}
	return filepath.Join(remoteGitRoot, rel), nil
}

// parseHostPort splits a host string of the form [user@]host[:port] into
// a clean host (without port) and a port number. Port 0 means unspecified.
func parseHostPort(host string) (cleanHost string, port int) {
	if i := strings.LastIndex(host, ":"); i > 0 {
		portStr := host[i+1:]
		if p, err := strconv.Atoi(portStr); err == nil && p > 0 && p < 65536 {
			return host[:i], p
		}
	}
	return host, 0
}

func sshPortArgs(port int) []string {
	if port == 0 {
		return nil
	}
	return []string{"-p", strconv.Itoa(port)}
}

// CheckSSH verifies that SSH can reach the remote host.
func CheckSSH(host string) error {
	cleanHost, port := parseHostPort(host)
	args := []string{"-o", "ConnectTimeout=5", "-o", "BatchMode=yes"}
	args = append(args, sshPortArgs(port)...)
	args = append(args, cleanHost, "true")

	var stderrBuf bytes.Buffer
	cmd := exec.Command("ssh", args...)
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		stderr := stderrBuf.String()
		if hint := sshHint(stderr, host); hint != "" {
			return fmt.Errorf("%s", hint)
		}
		return fmt.Errorf("cannot connect to %s: %s", host, strings.TrimSpace(stderr))
	}
	return nil
}

// SyncWithRetry syncs files with exponential backoff on transient errors.
func SyncWithRetry(remote config.Remote, localDir, remoteDir string, opts SyncOptions, retryCfg config.RetryConfig, warnMB int) error {
	rc := retryCfg.WithDefaults()

	warnLargeTransfer(localDir, warnMB)

	b := backoff.NewExponentialBackOff()
	b.InitialInterval = rc.InitialInterval.Duration
	b.MaxInterval = rc.MaxInterval.Duration
	b.Multiplier = rc.Multiplier
	b.MaxElapsedTime = 0

	attempt := 0
	return backoff.Retry(func() error {
		attempt++
		if attempt > 1 {
			log.Info("Retrying sync", "attempt", attempt, "of", rc.MaxAttempts)
		}
		err := doSync(remote, localDir, remoteDir, opts)
		if err == nil {
			return nil
		}
		if isTransient(err) && attempt < rc.MaxAttempts {
			log.Warn("Transient error, will retry", "err", err,
				"wait", rc.InitialInterval.Duration*time.Duration(attempt))
			if logging.File != nil {
				logging.File.Warn("transient sync error", "attempt", attempt, "err", err.Error())
			}
			return err
		}
		return backoff.Permanent(err)
	}, backoff.WithMaxRetries(b, uint64(rc.MaxAttempts-1)))
}

// doSync is the inner sync operation (no retry).
func doSync(remote config.Remote, localDir, remoteDir string, opts SyncOptions) error {
	tracked, err := exec.Command("git", "-C", localDir, "ls-files", "-z").Output()
	if err != nil {
		return fmt.Errorf("git ls-files: %w", err)
	}

	cleanHost, port := parseHostPort(remote.Host)

	if err := sshMkdir(cleanHost, port, remoteDir); err != nil {
		return fmt.Errorf("could not create remote directory %s: %w", remoteDir, err)
	}

	args := []string{"-az", "--files-from=-", "--from0"}
	if opts.Delete {
		args = append(args, "--delete")
	}
	if opts.Verbose {
		args = append(args, "-v")
	}
	if port != 0 {
		args = append(args, "-e", fmt.Sprintf("ssh -p %d", port))
	}
	args = append(args, localDir+"/", cleanHost+":"+remoteDir+"/")

	var stderrBuf bytes.Buffer
	cmd := exec.Command("rsync", args...)
	cmd.Stdin = strings.NewReader(string(tracked))
	cmd.Stdout = os.Stdout
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)

	// Warn if the sync is taking unexpectedly long.
	done := make(chan struct{})
	go func() {
		select {
		case <-done:
		case <-time.After(60 * time.Second):
			log.Warn("Sync is taking a while — large files tracked by git?",
				"tip", "consider .gitignore for datasets/models and pre-staging them on the remote")
		}
	}()
	runErr := cmd.Run()
	close(done)

	if runErr != nil {
		code := exitCode(runErr)
		stderr := stderrBuf.String()
		if logging.File != nil {
			logging.File.Error("rsync failed", "exit_code", code, "stderr", stderr)
		}
		return rsyncError(code, stderr, remote.Host)
	}
	return nil
}

// Run executes args on the remote inside remoteDir, with a PTY for live output.
func Run(remote config.Remote, remoteDir string, args []string) error {
	cleanHost, port := parseHostPort(remote.Host)

	escaped := make([]string, len(args))
	for i, a := range args {
		escaped[i] = shellescape(a)
	}
	remoteCmd := fmt.Sprintf("cd %s && %s", shellescape(remoteDir), strings.Join(escaped, " "))

	sshArgs := []string{"-t"}
	sshArgs = append(sshArgs, sshPortArgs(port)...)
	sshArgs = append(sshArgs, cleanHost, remoteCmd)

	var stderrBuf bytes.Buffer
	cmd := exec.Command("ssh", sshArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)

	if err := cmd.Run(); err != nil {
		code := exitCode(err)
		stderr := stderrBuf.String()
		if logging.File != nil {
			logging.File.Error("remote command failed",
				"exit_code", code,
				"cmd", strings.Join(args, " "))
		}
		// Exit code 1 from the remote process just means the command failed —
		// don't add SSH noise on top of whatever the remote already printed.
		if code == 255 {
			if hint := sshHint(stderr, remote.Host); hint != "" {
				return fmt.Errorf("SSH error: %s", hint)
			}
		}
		return fmt.Errorf("remote command exited with code %d", code)
	}
	return nil
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

	cleanHost, port := parseHostPort(remote.Host)
	sshArgs := sshPortArgs(port)
	sshArgs = append(sshArgs, cleanHost,
		fmt.Sprintf("cat > %s", shellescape(remoteDir+"/.rrun")))

	cmd := exec.Command("ssh", sshArgs...)
	cmd.Stdin = strings.NewReader(string(data) + "\n")
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// warnLargeTransfer checks local git-tracked file sizes and warns if large.
// This is local-only (no SSH overhead). warnMB: 0=default(100MB), -1=disabled.
func warnLargeTransfer(localDir string, warnMB int) {
	if warnMB < 0 {
		return
	}
	threshold := int64(warnMB) * 1024 * 1024
	if threshold == 0 {
		threshold = 100 * 1024 * 1024 // default 100 MB
	}

	out, err := exec.Command("git", "-C", localDir, "ls-files").Output()
	if err != nil {
		return
	}
	var total int64
	for _, rel := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if rel == "" {
			continue
		}
		info, err := os.Stat(filepath.Join(localDir, rel))
		if err == nil {
			total += info.Size()
		}
	}
	if total > threshold {
		log.Warn("Large sync detected",
			"total_size", formatBytes(total),
			"tip", "add large files to .gitignore and pre-stage them on the remote with rsync directly")
	}
}

// dryRunTransferSize queries rsync --dry-run to estimate the delta transfer size.
// Returns 0 on any failure (non-fatal).
func dryRunTransferSize(remote config.Remote, localDir, remoteDir string) int64 {
	tracked, err := exec.Command("git", "-C", localDir, "ls-files", "-z").Output()
	if err != nil {
		return 0
	}
	cleanHost, port := parseHostPort(remote.Host)
	args := []string{"-az", "--dry-run", "--stats", "--files-from=-", "--from0"}
	if port != 0 {
		args = append(args, "-e", fmt.Sprintf("ssh -p %d", port))
	}
	args = append(args, localDir+"/", cleanHost+":"+remoteDir+"/")

	cmd := exec.Command("rsync", args...)
	cmd.Stdin = strings.NewReader(string(tracked))
	out, _ := cmd.Output()

	re := regexp.MustCompile(`Total transferred file size:\s+([\d,]+)`)
	if m := re.FindSubmatch(out); m != nil {
		sizeStr := strings.ReplaceAll(string(m[1]), ",", "")
		size, _ := strconv.ParseInt(sizeStr, 10, 64)
		return size
	}
	return 0
}

func sshMkdir(host string, port int, dir string) error {
	args := sshPortArgs(port)
	args = append(args, host, "mkdir -p "+shellescape(dir))
	cmd := exec.Command("ssh", args...)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// isTransient returns true if the error looks like a recoverable network issue.
func isTransient(err error) bool {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	switch exitErr.ExitCode() {
	case 10, 30, 35: // rsync: socket I/O, timeout, daemon timeout
		return true
	}
	msg := err.Error()
	for _, pat := range []string{
		"Broken pipe", "Connection reset", "timed out",
		"No route to host", "Network is unreachable",
	} {
		if strings.Contains(msg, pat) {
			return true
		}
	}
	return false
}

// rsyncError constructs a helpful error from an rsync failure.
func rsyncError(code int, stderr, host string) error {
	switch code {
	case 255:
		if hint := sshHint(stderr, host); hint != "" {
			return fmt.Errorf("SSH error: %s", hint)
		}
		return fmt.Errorf("SSH connection failed (exit 255) — check that %s is reachable", host)
	case 23:
		if strings.Contains(stderr, "Permission denied") {
			return fmt.Errorf("permission denied writing some files on the remote — check ownership/permissions")
		}
		return fmt.Errorf("partial transfer: some files could not be written on the remote (exit 23)")
	case 11:
		return fmt.Errorf("file I/O error on the remote — disk full? (exit 11)")
	case 10:
		return fmt.Errorf("network I/O error — connection interrupted (exit 10)")
	case 30:
		return fmt.Errorf("connection timed out during transfer (exit 30)")
	}
	return fmt.Errorf("rsync failed with exit code %d", code)
}

// sshHint returns a human-friendly message for SSH connection errors.
func sshHint(stderr, host string) string {
	switch {
	case strings.Contains(stderr, "Connection refused"):
		return fmt.Sprintf("connection refused — is sshd running on %s? (sudo systemctl start sshd)", host)
	case strings.Contains(stderr, "Permission denied (publickey"):
		return fmt.Sprintf("SSH key rejected — copy your key with: ssh-copy-id %s", host)
	case strings.Contains(stderr, "Host key verification failed"):
		return fmt.Sprintf("host key mismatch — if the host was reinstalled, run: ssh-keygen -R %s", host)
	case strings.Contains(stderr, "No route to host"):
		return fmt.Sprintf("no route to %s — is the host reachable?", host)
	case strings.Contains(stderr, "Connection timed out"):
		return fmt.Sprintf("connection to %s timed out", host)
	case strings.Contains(stderr, "Could not resolve hostname"):
		return fmt.Sprintf("hostname %q could not be resolved — check the hostname or your SSH config", host)
	}
	return ""
}

func exitCode(err error) int {
	var e *exec.ExitError
	if errors.As(err, &e) {
		return e.ExitCode()
	}
	return -1
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

func shellescape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
