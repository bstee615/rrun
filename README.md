# rrun: **r**emote **run**ner

<p align="center">
  <img src="logo.svg" alt="rrun" width="400"/>
</p>

Sync git-tracked files to a remote machine and run commands there with live-streamed output. Zero project buy-in — works with any git repo, no config files added to your projects.

Built on `rsync` and `ssh`. Files on the remote are overwritten on sync but never deleted.

## How it works

1. Runs `git ls-files` to get the list of tracked files
2. Rsyncs those files to the remote (mirroring or remapping the local path)
3. Writes a `.rrun` metadata file to the remote directory
4. Optionally SSHes in and runs your command, streaming output live

## Build

```sh
git clone <this repo>
cd rrun
go build -o rrun .
```

## Install

```sh
go install .
# or copy the binary wherever you like
cp rrun ~/bin/rrun
```

Requires `rsync` and `ssh` on your local machine, and `rsync` on the remote.

## Setup

Add a named remote once. `<host>` is anything SSH accepts — a bare hostname, an alias from `~/.ssh/config`, or `user@host`.

```sh
# Mirror the local path exactly on the remote
rrun remote add workstation gpu-box

# Map a path prefix (local /home/you → remote /home/gpu-user)
rrun remote add workstation gpu-box \
  --local-path /home/you \
  --remote-path /home/gpu-user

# The first remote added becomes the default automatically
rrun remote list
rrun remote default workstation
```

Config is stored at `~/.config/rrun/config.yaml`.

## Usage

```sh
# Sync git-tracked files to the default remote
rrun sync

# Sync then run a command (output streams live)
rrun run python train.py --epochs 10

# Target a specific remote
rrun run --remote workstation make test

# Flags before the command; everything after the first positional arg
# is passed through to the remote shell as-is
rrun run --remote workstation bash -c 'nvidia-smi && python bench.py'
```

### Remote management

```sh
rrun remote add <name> <host> [--local-path <path>] [--remote-path <path>]
rrun remote remove <name>
rrun remote list
rrun remote show <name>
rrun remote default <name>
```

## Self-copy test (localhost)

Start your local SSH daemon and authorize your own key, then add localhost as a remote with a path remap:

```sh
sudo systemctl start sshd
ssh-keygen -q -N '' -f ~/.ssh/id_ed25519 2>/dev/null || true
cat ~/.ssh/id_ed25519.pub >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys

rrun remote add localtest localhost \
  --local-path /home/you/Code \
  --remote-path /tmp/rrun-dst
```

Then from any git repo under `~/Code`:

```sh
cd rrun/example
rrun run python train.py
```

You should see the files appear under `/tmp/rrun-dst/<repo-name>/` and the script run remotely with live output. The `.rrun` file written there records where the sync came from:

```json
{
  "source_machine": "your-laptop",
  "source_path": "/home/you/Code/rrun/example",
  "last_sync": "2026-03-22T20:23:52Z",
  "last_command": "python train.py"
}
```

## Tests

```sh
go test ./...
```

> No tests yet — contributions welcome.

## Config file format

`~/.config/rrun/config.yaml`:

```yaml
default_remote: workstation

remotes:
  workstation:
    host: gpu-box
    path_map:
      local: /home/you
      remote: /home/gpu-user
  lambda:
    host: ubuntu@1.2.3.4
    path_map:
      local: /home/you/Code
      remote: /home/ubuntu/Code
```

If `path_map` is omitted, the local absolute path is mirrored exactly on the remote.

## .rrun state file

After each sync, `rrun` writes a `.rrun` JSON file to the root of the synced directory on the remote. It is never synced back and is excluded from rsync so it persists across syncs.

| Field | Description |
|---|---|
| `source_machine` | Hostname of the machine that last synced |
| `source_path` | Absolute local path that was synced |
| `last_sync` | UTC timestamp of the last sync |
| `last_command` | Last command passed to `rrun run` (empty for `rrun sync`) |
