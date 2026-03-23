# Contributing & Development

## Prerequisites

- Go 1.22+
- `rsync`, `ssh` (runtime deps, needed for tests)
- [`nfpm`](https://nfpm.goreleaser.com/) — for building .deb/.rpm packages
- [`gh`](https://cli.github.com/) — for publishing GitHub releases

## Development workflow

Build once and symlink the binary into your PATH so subsequent builds take effect immediately without sudo:

```sh
make dev
```

After that, rebuilding is just:

```sh
go build .
# or
make build
```

## Makefile targets

| Target | Description |
|---|---|
| `make build` | Compile the binary to `./rrun` |
| `make dev` | Build and symlink `/usr/local/bin/rrun → ./rrun` |
| `make test` | Run `go test ./...` |
| `make deb` | Build a `.deb` package into `dist/` |
| `make rpm` | Build a `.rpm` package into `dist/` |
| `make packages` | Build both `.deb` and `.rpm` |
| `make install` | Install via the appropriate package manager (deb/rpm/binary) |
| `make install-deb` | Install the `.deb` with `dpkg` |
| `make install-rpm` | Install the `.rpm` with `rpm` |
| `make release` | Clean → test → packages → publish GitHub release |
| `make clean` | Remove the binary and `dist/` |

## Packaging

### Debian / Ubuntu (`.deb`) and Fedora / RHEL (`.rpm`)

Packages are built with [nfpm](https://nfpm.goreleaser.com/) using `nfpm.yaml` as the config. The `VERSION` variable is derived from `git describe --tags --always --dirty`.

```sh
make deb    # produces dist/rrun_<version>_amd64.deb
make rpm    # produces dist/rrun-<version>-1.x86_64.rpm
make packages  # both
```

### Arch Linux (AUR)

The `PKGBUILD` is maintained in this repo and published to the AUR separately. After tagging a release:

1. Update `pkgver` and `sha256sums` in `PKGBUILD`.
2. Push to the AUR git remote:

```sh
git clone ssh://aur@aur.archlinux.org/rrun.git aur-rrun
cp PKGBUILD aur-rrun/
cd aur-rrun
makepkg --printsrcinfo > .SRCINFO
git add PKGBUILD .SRCINFO
git commit -m "Update to <version>"
git push
```

## Releasing

Tag the commit, then run:

```sh
git tag v<version>
git push origin v<version>
make release
```

`make release` runs tests, builds `.deb` and `.rpm`, then calls `gh release create` with auto-generated notes from commit history. After the GitHub release is published, update the AUR `PKGBUILD` as described above.
