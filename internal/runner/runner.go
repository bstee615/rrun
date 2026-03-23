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
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/charmbracelet/log"

	"github.com/bstee615/rrun/internal/config"
	"github.com/bstee615/rrun/internal/logging"
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

// Conn holds a persistent SSH connection to a remote host. All rrun operations
// share this connection via SSH ControlMaster, so the user is prompted for a
// password (if no key is configured) exactly once per rrun invocation.
type Conn struct {
	remote      config.Remote
	cleanHost   string
	port        int
	controlPath string // empty when ControlMaster is not in use
	tempDir     string
}

// Dial connects to remote, establishing an SSH ControlMaster. If public-key
// auth is not available the user is prompted for a password here — once —
// and all subsequent operations reuse the same underlying TCP connection.
func Dial(remote config.Remote) (*Conn, error) {
	cleanHost, port := ParseHostPort(remote.Host)
	c := &Conn{remote: remote, cleanHost: cleanHost, port: port}

	dir, err := os.MkdirTemp("", "rrun-*")
	if err != nil {
		// Can't get a temp dir; fall back to per-connection auth.
		return c, nil
	}
	c.tempDir = dir
	// Short socket name — some systems have tight Unix socket path limits.
	c.controlPath = filepath.Join(dir, "s")

	args := []string{
		"-o", "ControlMaster=yes",
		"-o", "ControlPath=" + c.controlPath,
		"-o", "ControlPersist=yes", // master stays alive after we exit
		"-o", "ConnectTimeout=10",
		"-N", // no remote command; exit once the master is ready
	}
	args = append(args, sshPortArgs(port)...)
	args = append(args, cleanHost)

	var stderrBuf bytes.Buffer
	cmd := exec.Command("ssh", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)

	if err := cmd.Run(); err != nil {
		stderr := stderrBuf.String()
		c.removeTemp()
		if hint := sshHint(stderr, remote.Host); hint != "" {
			return nil, errors.New(hint)
		}
		return nil, fmt.Errorf("cannot connect to %s", remote.Host)
	}
	return c, nil
}

// Close shuts down the ControlMaster and removes the temp socket directory.
func (c *Conn) Close() {
	if c.controlPath != "" {
		exec.Command("ssh", "-O", "exit", // tell the master to exit
			"-o", "ControlPath="+c.controlPath,
			c.cleanHost).Run()
	}
	c.removeTemp()
}

func (c *Conn) removeTemp() {
	if c.tempDir != "" {
		os.RemoveAll(c.tempDir)
		c.tempDir = ""
		c.controlPath = ""
	}
}

// sshArgs returns the extra flags that route an ssh command through the
// control socket. Empty when no ControlMaster is in use.
func (c *Conn) sshArgs() []string {
	if c.controlPath == "" {
		return nil
	}
	return []string{
		"-o", "ControlPath=" + c.controlPath,
		"-o", "ControlMaster=no",
	}
}

// rsyncSSHCmd builds the -e argument for rsync so its internal ssh call uses
// the control socket (and the correct port, if non-default).
func (c *Conn) rsyncSSHCmd() string {
	parts := []string{"ssh"}
	if c.port != 0 {
		parts = append(parts, "-p", strconv.Itoa(c.port))
	}
	if c.controlPath != "" {
		parts = append(parts,
			"-o", "ControlPath="+c.controlPath,
			"-o", "ControlMaster=no")
	}
	return strings.Join(parts, " ")
}

// CheckDeps verifies that the required external binaries are in PATH.
func CheckDeps() error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("rrun requires rsync and ssh, which are not natively available on Windows — run under WSL2 instead")
	}
	for _, bin := range []string{"rsync", "ssh"} {
		if _, err := exec.LookPath(bin); err != nil {
			return fmt.Errorf("%s not found in PATH — install it via your package manager", bin)
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

// ParseHostPort splits a host string of the form [user@]host[:port] into
// a clean host (without port) and a port number. Port 0 means unspecified.
func ParseHostPort(host string) (cleanHost string, port int) {
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

// Sync syncs files to the remote with exponential backoff on transient errors.
func (c *Conn) Sync(localDir, remoteDir string, opts SyncOptions, retryCfg config.RetryConfig, warnMB int) error {
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
		err := c.doSync(localDir, remoteDir, opts)
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
func (c *Conn) doSync(localDir, remoteDir string, opts SyncOptions) error {
	tracked, err := exec.Command("git", "-C", localDir, "ls-files", "-z").Output()
	if err != nil {
		return fmt.Errorf("git ls-files: %w", err)
	}

	if err := c.mkdir(remoteDir); err != nil {
		return fmt.Errorf("could not create remote directory %s: %w", remoteDir, err)
	}

	args := []string{"-az", "--files-from=-", "--from0"}
	if opts.Delete {
		args = append(args, "--delete")
	}
	if opts.Verbose {
		args = append(args, "-v")
	}
	// Always pass -e so we can inject the control socket (and port if needed).
	args = append(args, "-e", c.rsyncSSHCmd())
	args = append(args, localDir+"/", c.cleanHost+":"+remoteDir+"/")

	var stderrBuf bytes.Buffer
	cmd := exec.Command("rsync", args...)
	cmd.Stdin = strings.NewReader(string(tracked))
	cmd.Stdout = os.Stdout
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)

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
		return rsyncError(code, stderr, c.remote.Host)
	}
	return nil
}

// Run executes args on the remote inside remoteDir, with a PTY for live output.
func (c *Conn) Run(remoteDir string, args []string) error {
	escaped := make([]string, len(args))
	for i, a := range args {
		escaped[i] = shellescape(a)
	}
	remoteCmd := fmt.Sprintf("bash -l -c %s",
		shellescape(fmt.Sprintf("cd %s && %s", remoteDir, strings.Join(escaped, " "))))

	sshArgs := c.sshArgs()
	sshArgs = append(sshArgs, "-t")
	sshArgs = append(sshArgs, sshPortArgs(c.port)...)
	sshArgs = append(sshArgs, c.cleanHost, remoteCmd)

	var stderrBuf bytes.Buffer
	cmd := exec.Command("ssh", sshArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)

	if err := cmd.Run(); err != nil {
		code := exitCode(err)
		stderr := stderrBuf.String()
		if logging.File != nil {
			logging.File.Error("remote command failed", "exit_code", code, "cmd", strings.Join(args, " "))
		}
		if code == 255 {
			if hint := sshHint(stderr, c.remote.Host); hint != "" {
				return fmt.Errorf("SSH error: %s", hint)
			}
		}
		return fmt.Errorf("remote command exited with code %d", code)
	}
	return nil
}

// WriteState writes a .rrun metadata file to remoteDir on the remote.
func (c *Conn) WriteState(localDir, remoteDir, lastCmd string) error {
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

	sshArgs := c.sshArgs()
	sshArgs = append(sshArgs, sshPortArgs(c.port)...)
	sshArgs = append(sshArgs, c.cleanHost,
		fmt.Sprintf("cat > %s", shellescape(remoteDir+"/.rrun")))

	cmd := exec.Command("ssh", sshArgs...)
	cmd.Stdin = strings.NewReader(string(data) + "\n")
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// mkdir creates dir on the remote, creating parent directories as needed.
func (c *Conn) mkdir(dir string) error {
	args := c.sshArgs()
	args = append(args, sshPortArgs(c.port)...)
	args = append(args, c.cleanHost, "mkdir -p "+shellescape(dir))
	cmd := exec.Command("ssh", args...)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// warnLargeTransfer checks local git-tracked file sizes and warns if large.
func warnLargeTransfer(localDir string, warnMB int) {
	if warnMB < 0 {
		return
	}
	threshold := int64(warnMB) * 1024 * 1024
	if threshold == 0 {
		threshold = 100 * 1024 * 1024
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

// Shellescape wraps s in single quotes, escaping embedded single quotes.
func Shellescape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func shellescape(s string) string { return Shellescape(s) }
