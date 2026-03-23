#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
DIST="$ROOT/dist"
PASS=0
FAIL=0

run_test() {
    local name="$1"
    local image="$2"
    local cmd="$3"

    printf "  %-30s" "$name"
    if docker run --rm -v "$DIST:/dist" "$image" bash -c "$cmd" >/dev/null 2>&1; then
        echo "PASS"
        PASS=$((PASS + 1))
    else
        echo "FAIL"
        FAIL=$((FAIL + 1))
    fi
}

DEB=$(ls "$DIST"/*.deb 2>/dev/null | head -1)
RPM=$(ls "$DIST"/*.rpm 2>/dev/null | head -1)

if [[ -z "$DEB" || -z "$RPM" ]]; then
    echo "error: packages not found in dist/ — run 'make packages' first" >&2
    exit 1
fi

DEB_FILE="/dist/$(basename "$DEB")"
RPM_FILE="/dist/$(basename "$RPM")"

echo "Testing .deb packages..."
run_test "ubuntu:24.04" "ubuntu:24.04" \
    "apt-get update -qq && apt-get install -y $DEB_FILE && rrun --version"
run_test "ubuntu:22.04" "ubuntu:22.04" \
    "apt-get update -qq && apt-get install -y $DEB_FILE && rrun --version"
run_test "debian:12" "debian:12" \
    "apt-get update -qq && apt-get install -y $DEB_FILE && rrun --version"

echo "Testing .rpm packages..."
run_test "fedora:41" "fedora:41" \
    "dnf install -y $RPM_FILE && rrun --version"
run_test "fedora:40" "fedora:40" \
    "dnf install -y $RPM_FILE && rrun --version"

echo "Testing Arch Linux PKGBUILD..."
test_arch() {
    printf "  %-30s" "archlinux:latest"

    local tmpdir
    tmpdir=$(mktemp -d)
    trap "rm -rf '$tmpdir'" RETURN

    # Create a source tarball from the local repo so the test doesn't need a
    # published release. makepkg will extract it into rrun-<pkgver>/.
    local pkgver
    pkgver=$(git -C "$ROOT" describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || echo "0.1.0")
    local tarball="rrun-${pkgver}.tar.gz"
    git -C "$ROOT" archive --prefix="rrun-${pkgver}/" HEAD | gzip > "$tmpdir/$tarball"
    local sha256
    sha256=$(sha256sum "$tmpdir/$tarball" | cut -d' ' -f1)

    # Rewrite PKGBUILD to use the local tarball instead of the GitHub URL
    sed \
        -e "s|^pkgver=.*|pkgver=$pkgver|" \
        -e "s|^source=.*|source=(\"$tarball\")|" \
        -e "s|^sha256sums=.*|sha256sums=('$sha256')|" \
        "$ROOT/PKGBUILD" > "$tmpdir/PKGBUILD"

    if docker run --rm \
        -v "$tmpdir:/build" \
        archlinux:latest \
        bash -c "
            pacman -Syu --noconfirm --needed base-devel go &&
            useradd -m builder &&
            chown -R builder:builder /build &&
            su builder -c 'cd /build && makepkg --noconfirm --nodeps' &&
            pacman -U --noconfirm /build/rrun-*.pkg.tar.zst &&
            rrun --version
        " >/dev/null 2>&1; then
        echo "PASS"
        PASS=$((PASS + 1))
    else
        echo "FAIL"
        FAIL=$((FAIL + 1))
    fi
}
test_arch

echo ""
echo "Results: $PASS passed, $FAIL failed"
[[ $FAIL -eq 0 ]]
