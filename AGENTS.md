# rrun — Agent Guide

This file documents `rrun` for AI coding agents. It covers what the tool does,
how to use it from within a project, and how to work on the rrun codebase itself.

---

## What rrun does

`rrun` syncs git-tracked files from a local machine to a named remote (via rsync
over SSH) and optionally runs a command on the remote with live-streamed output.
It is configured once globally and works from any git repository.

---

## Using rrun from another project

When working in a project on behalf of a user who has rrun installed and configured,
you can use the following commands to sync and run things on their remote GPU machine.

### Check what remotes are configured

```sh
rrun remote list
```

### Sync the current project to the default remote

```sh
rrun sync
```

### Sync and run a command

```sh
rrun run python train.py
rrun run make test
rrun run --remote workstation pytest tests/
```

### Useful flags

| Flag | Effect |
|---|---|
| `--remote <name>` | Use a specific remote instead of the default |
| `--delete` | Remove remote files no longer tracked by git (clean sync) |
| `--quiet` | Suppress info/warn messages |
| `--verbose` | Show each file rsync transfers |
| `--no-state` | Don't write the `.rrun` metadata file |

### When to use --delete

Use `--delete` when the remote directory needs to be brought into exact parity
with the local git tree — for example after renaming or deleting tracked files.
Without it, rrun never removes files from the remote.

---

## Working on the rrun codebase

### Project structure

```
rrun/
  main.go                    entry point
  cmd/
    root.go                  global flags, PersistentPreRunE, version
    run.go                   `rrun run` subcommand
    sync.go                  `rrun sync` subcommand
    remote.go                `rrun remote *` subcommands
    helpers.go               shared resolveRemote(), syncArgs()
  internal/
    config/config.go         Config struct, Load/Save, Duration type
    runner/runner.go         rsync/ssh execution, retry, error handling
    logging/logging.go       file logger setup (lumberjack + slog)
    sshconf/sshconf.go       ~/.ssh/config parsing for remote show
  example/
    train.py                 self-test example
  logo.svg
  PKGBUILD                   Arch Linux packaging
  CHANGELOG.md
```

### Key design rules

- **No project buy-in**: rrun never modifies files in the user's project directory.
- **SSH config is canonical**: rrun stores only the alias/host string; all SSH settings
  (port, user, key) are read by SSH itself from `~/.ssh/config`. rrun just surfaces
  them in `remote show` for convenience.
- **Only tracked files sync**: rsync is fed from `git ls-files -z`, so untracked files,
  build artifacts, and gitignored data never transfer.
- **`.rrun` on remote only**: the state file is written via SSH and never synced back.

### Building

```sh
go build -ldflags "-X rrun/cmd.version=dev" -o rrun .
```

### Running the self-test (localhost)

```sh
# Ensure sshd is running and localhost is authorized:
sudo systemctl start sshd

# Add a localhost remote with a path remap:
rrun remote add localtest localhost \
  --local-path /home/you/Code \
  --remote-path /tmp/rrun-dst

# From the example project:
cd example
rrun run python train.py
```

### Adding a new subcommand

1. Add a new `*cobra.Command` var in the appropriate `cmd/*.go` file.
2. Register it in `init()` with `parentCmd.AddCommand(newCmd)`.
3. Load config with `config.Load()` and call runner functions from `internal/runner`.
4. Use `log.Info/Warn/Error` for user-facing messages (suppressed by `--quiet` where appropriate).
5. Log debug detail to `logging.File` (writes to the persistent log file).

### Error handling conventions

- Runner functions return descriptive errors; cmd functions wrap with context if needed.
- SSH/rsync errors are translated to actionable hints in `runner.sshHint` and `runner.rsyncError`.
- Transient errors (network I/O, timeout) trigger automatic retry via exponential backoff.
- Always return errors to cobra rather than `os.Exit` directly.
