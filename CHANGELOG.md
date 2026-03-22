# Changelog

## v0.1.0 — 2026-03-22

Initial release.

### Features

- **`rrun sync`** — sync git-tracked files to a named remote via rsync over SSH.
  Files on the remote are overwritten but never deleted unless `--delete` is passed.
- **`rrun run [command...]`** — sync then execute a command on the remote with live-streamed output (PTY).
- **Named remotes** — configure once in `~/.config/rrun/config.yaml`, use from any git repo.
- **Path mapping** — remap local path prefixes to different remote paths per-remote.
- **SSH config integration** — host aliases from `~/.ssh/config` work transparently.
  Specify host as an SSH alias, `user@host`, or `user@host:port`.
- **`rrun remote` subcommands** — mirrors git's remote interface:
  `add`, `remove`, `rename`, `list`, `show`, `default`, `get-url`, `set-url`, `set-path`.
- **Retry on transient errors** — exponential backoff for network interruptions,
  configurable via `retry:` in config.
- **Large transfer warnings** — warns before syncing when git-tracked files exceed a threshold (default 100 MiB).
- **Slow sync warning** — warns if rsync takes longer than 60 seconds.
- **Clean error messages** — SSH errors (key rejected, host unreachable, key mismatch, etc.)
  are translated into actionable hints rather than raw SSH output.
- **Dependency checks** — clear error if `rsync` or `ssh` are not installed.
- **Persistent debug log** — structured JSON log at `~/.local/share/rrun/rrun.log`
  (XDG_DATA_HOME compliant), with rotation via lumberjack.
- **`.rrun` state file** — written to the remote after each sync: source machine,
  source path, last sync time, last command.
- **`--delete`** — pass rsync `--delete` to remove remote files not tracked by git.
- **`--no-state`** — skip writing the `.rrun` file.
- **`--quiet`** — suppress info/warn messages (errors still shown).
- **`--verbose`** — pass `-v` to rsync for per-file output.

### Config file reference

`~/.config/rrun/config.yaml`:

```yaml
default_remote: workstation
quiet: false
no_state: false
large_transfer_warn_mb: 100   # 0 = default (100), -1 = disabled

retry:
  max_attempts: 3
  initial_interval: 2s
  max_interval: 30s
  multiplier: 2.0

remotes:
  workstation:
    host: gpu-box             # SSH alias, user@host, or user@host:port
    path_map:
      local: /home/you
      remote: /home/you
```
