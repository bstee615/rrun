#!/usr/bin/env bash
# Publish rrun to Launchpad PPA.
# Prerequisites:
#   - GPG key for benjaminjsteenhoek@gmail.com imported and uploaded to Launchpad
#   - sudo apt install devscripts dput debhelper golang-go
#   - PPA created at https://launchpad.net/~bstee615/+activate-ppa
# Usage:
#   ./release-ppa.sh               # releases to noble only
#   DISTROS="noble jammy" ./release-ppa.sh
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
PPA="${PPA:-ppa:bstee615/rrun}"
DISTROS="${DISTROS:-noble}"

die() { echo "error: $*" >&2; exit 1; }

# ── version ───────────────────────────────────────────────────────────────────
VERSION_TAG=$(git -C "$ROOT" describe --tags --exact-match 2>/dev/null) \
    || die "HEAD is not an exact tag."
PKGVER="${VERSION_TAG#v}"

# ── prereq checks ─────────────────────────────────────────────────────────────
for cmd in dput dch dpkg-buildpackage gpg curl go; do
    command -v "$cmd" >/dev/null 2>&1 \
        || die "'$cmd' not found. Install: sudo apt install devscripts dput debhelper golang-go"
done

MAINTAINER_EMAIL="benjaminjsteenhoek@gmail.com"
GPG_KEY=$(gpg --list-secret-keys --with-colons "$MAINTAINER_EMAIL" 2>/dev/null \
    | awk -F: '/^sec/{print $5; exit}')
[[ -n "$GPG_KEY" ]] \
    || die "No GPG key for $MAINTAINER_EMAIL. See: https://help.launchpad.net/YourAccount/ImportingYourPGPKey"

# ── build area ────────────────────────────────────────────────────────────────
BUILDDIR=$(mktemp -d)
trap 'rm -rf "$BUILDDIR"' EXIT

SRCDIR="$BUILDDIR/rrun-${PKGVER}"
ORIGNAME="rrun_${PKGVER}.orig.tar.gz"

# Export source + vendor Go modules (Launchpad blocks network during builds)
echo "Exporting source and vendoring Go modules ..."
git -C "$ROOT" archive --prefix="rrun-${PKGVER}/" HEAD | tar -x -C "$BUILDDIR"
( cd "$SRCDIR" && go mod vendor )

# Create orig tarball
tar -czf "$BUILDDIR/$ORIGNAME" -C "$BUILDDIR" "rrun-${PKGVER}"
echo "Created $ORIGNAME ($(du -sh "$BUILDDIR/$ORIGNAME" | cut -f1))"

# ── upload per distro ─────────────────────────────────────────────────────────
for DISTRO in $DISTROS; do
    echo ""
    echo "=== $DISTRO ==="

    DISTRO_DIR="$BUILDDIR/$DISTRO"
    mkdir -p "$DISTRO_DIR"
    cp -a "$SRCDIR" "$DISTRO_DIR/rrun-${PKGVER}"
    cp "$BUILDDIR/$ORIGNAME" "$DISTRO_DIR/$ORIGNAME"
    cp -r "$ROOT/debian" "$DISTRO_DIR/rrun-${PKGVER}/debian"

    DEBIAN_VERSION="${PKGVER}-1~${DISTRO}1"
    cd "$DISTRO_DIR/rrun-${PKGVER}"

    dch --newversion "$DEBIAN_VERSION" \
        --distribution "$DISTRO" \
        --force-distribution \
        "Release ${PKGVER} for ${DISTRO}."
    dch --release ""

    dpkg-buildpackage -S -sa -k"$GPG_KEY" --no-check-builddeps

    CHANGES="$DISTRO_DIR/rrun_${DEBIAN_VERSION}_source.changes"
    [[ -f "$CHANGES" ]] || die "Expected .changes not found: $CHANGES"

    echo "Uploading to $PPA ..."
    dput "$PPA" "$CHANGES"
    echo "$DISTRO: upload complete"
    cd "$ROOT"
done

echo ""
echo "PPA: released rrun v${PKGVER} for: $DISTROS"
echo "Monitor: https://launchpad.net/~bstee615/+archive/ubuntu/rrun/+packages"
