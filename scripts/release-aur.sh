#!/usr/bin/env bash
# Publish rrun to the AUR.
# Prerequisites:
#   - SSH key registered at https://aur.archlinux.org/account (under SSH Keys)
#   - Key loaded in ssh-agent: ssh-add ~/.ssh/aur
#   - Docker (for makepkg --printsrcinfo)
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
die() { echo "error: $*" >&2; exit 1; }

# ── version ───────────────────────────────────────────────────────────────────
VERSION_TAG=$(git -C "$ROOT" describe --tags --exact-match 2>/dev/null) \
    || die "HEAD is not an exact tag. Run: git tag vX.Y.Z && git push --tags"
PKGVER="${VERSION_TAG#v}"

# ── prereq checks ─────────────────────────────────────────────────────────────
command -v docker >/dev/null 2>&1 || die "docker not found"
ssh-add -l >/dev/null 2>&1      || die "No SSH keys in agent. Run: ssh-add ~/.ssh/aur"

SSH_OUT=$(ssh -o BatchMode=yes -o ConnectTimeout=10 \
    aur@aur.archlinux.org help 2>&1 || true)
echo "$SSH_OUT" | grep -qi "permission denied" \
    && die "AUR SSH key rejected. Register your key at https://aur.archlinux.org/account"

# ── fetch tarball + sha256 ────────────────────────────────────────────────────
TARBALL_URL="https://github.com/bstee615/rrun/archive/${VERSION_TAG}.tar.gz"
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

echo "Fetching $TARBALL_URL ..."
curl -fsSL "$TARBALL_URL" -o "$TMPDIR/rrun-${PKGVER}.tar.gz" \
    || die "Download failed. Is the GitHub release published?"
SHA256=$(sha256sum "$TMPDIR/rrun-${PKGVER}.tar.gz" | cut -d' ' -f1)
echo "sha256: $SHA256"

# ── clone AUR repo + update PKGBUILD ─────────────────────────────────────────
AUR_DIR="$TMPDIR/aur-rrun"
git clone ssh://aur@aur.archlinux.org/rrun.git "$AUR_DIR" \
    || die "Failed to clone AUR repo. Is the package registered at https://aur.archlinux.org/packages/rrun?"

cp "$ROOT/PKGBUILD" "$AUR_DIR/PKGBUILD"
sed -i \
    -e "s|^pkgver=.*|pkgver=${PKGVER}|" \
    -e "s|^pkgrel=.*|pkgrel=1|" \
    -e "s|^sha256sums=.*|sha256sums=('${SHA256}')|" \
    "$AUR_DIR/PKGBUILD"

# ── regenerate .SRCINFO via Arch Docker ───────────────────────────────────────
echo "Regenerating .SRCINFO ..."
docker run --rm -v "$AUR_DIR:/pkg" archlinux:latest bash -c "
    pacman -Sy --noconfirm --needed base-devel >/dev/null 2>&1
    useradd -m builder
    chown -R builder:builder /pkg
    su builder -c 'cd /pkg && makepkg --printsrcinfo > .SRCINFO'
"

# ── commit + push ─────────────────────────────────────────────────────────────
git -C "$AUR_DIR" add PKGBUILD .SRCINFO
git -C "$AUR_DIR" diff --cached --quiet \
    && die "No changes — is v${PKGVER} already on AUR?"
git -C "$AUR_DIR" commit -m "Update to v${PKGVER}"
git -C "$AUR_DIR" push origin master

echo "AUR: released rrun v${PKGVER}"
