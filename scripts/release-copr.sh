#!/usr/bin/env bash
# Publish rrun to Fedora COPR.
# Prerequisites:
#   - pip install copr-cli  (or: dnf install copr-cli)
#   - API token at ~/.config/copr (download from https://copr.fedorainfracloud.org/api/)
#   - COPR project created at https://copr.fedorainfracloud.org/coprs/add/
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
COPR_PROJECT="${COPR_PROJECT:-bstee615/rrun}"

die() { echo "error: $*" >&2; exit 1; }

# ── version ───────────────────────────────────────────────────────────────────
VERSION_TAG=$(git -C "$ROOT" describe --tags --exact-match 2>/dev/null) \
    || die "HEAD is not an exact tag."
PKGVER="${VERSION_TAG#v}"

# ── prereq checks ─────────────────────────────────────────────────────────────
command -v copr-cli >/dev/null 2>&1 \
    || die "copr-cli not found. Install: pip install copr-cli"
[[ -f "$HOME/.config/copr" ]] \
    || die "~/.config/copr not found. Download token from https://copr.fedorainfracloud.org/api/"

# ── submit build ──────────────────────────────────────────────────────────────
# build-scm clones the GitHub repo at the given tag and builds with rrun.spec.
# --enable-net=on allows go module downloads inside the mock build environment.
echo "Submitting COPR build for ${COPR_PROJECT} @ ${VERSION_TAG} ..."
copr-cli build-scm \
    "$COPR_PROJECT" \
    --clone-url "https://github.com/bstee615/rrun" \
    --commit "$VERSION_TAG" \
    --spec "rrun.spec" \
    --type "git" \
    --enable-net=on \
    --nowait

echo "COPR: build submitted for rrun v${PKGVER}"
echo "Monitor: https://copr.fedorainfracloud.org/coprs/${COPR_PROJECT}/builds/"
