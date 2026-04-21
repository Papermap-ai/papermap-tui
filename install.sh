#!/usr/bin/env sh
# Papermap TUI installer.
#
# Downloads the latest papermap-tui release for your OS/arch from GitHub
# Releases, verifies its SHA256 checksum, and installs the binary to
# $PREFIX/bin (default: /usr/local or ~/.local).
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/papermap/papermap-tui/main/install.sh | sh
#
# Environment overrides:
#   PREFIX   Installation root (default /usr/local or ~/.local)
#   VERSION  Release tag to install (default: latest)
#   REPO     GitHub repo (default papermap/papermap-tui)

set -eu

REPO="${REPO:-papermap/papermap-tui}"
VERSION="${VERSION:-latest}"

err() {
    printf 'error: %s\n' "$1" >&2
    exit 1
}

info() {
    printf '==> %s\n' "$1"
}

require() {
    command -v "$1" >/dev/null 2>&1 || err "missing required command: $1"
}

require uname
require mkdir
require chmod
require tar

# Pick downloader.
DL=""
if command -v curl >/dev/null 2>&1; then
    DL="curl -fsSL"
elif command -v wget >/dev/null 2>&1; then
    DL="wget -qO-"
else
    err "need curl or wget"
fi

# Detect OS.
os_raw="$(uname -s)"
case "$os_raw" in
    Linux)  OS="linux" ;;
    Darwin) OS="darwin" ;;
    *) err "unsupported OS: $os_raw" ;;
esac

# Detect arch.
arch_raw="$(uname -m)"
case "$arch_raw" in
    x86_64|amd64) ARCH="x86_64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *) err "unsupported arch: $arch_raw" ;;
esac

# Resolve version tag.
if [ "$VERSION" = "latest" ]; then
    info "Resolving latest release..."
    api_url="https://api.github.com/repos/$REPO/releases/latest"
    TAG="$($DL "$api_url" | grep '"tag_name":' | head -n1 | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
    [ -n "$TAG" ] || err "failed to resolve latest release tag"
else
    TAG="$VERSION"
fi

VER="${TAG#v}"
ARCHIVE="papermap_${VER}_${OS}_${ARCH}.tar.gz"
BASE_URL="https://github.com/$REPO/releases/download/$TAG"
ARCHIVE_URL="$BASE_URL/$ARCHIVE"
CHECKSUMS_URL="$BASE_URL/checksums.txt"

info "Installing papermap $TAG ($OS/$ARCH)"

TMP="$(mktemp -d 2>/dev/null || mktemp -d -t papermap)"
trap 'rm -rf "$TMP"' EXIT INT TERM

info "Downloading $ARCHIVE"
$DL "$ARCHIVE_URL" > "$TMP/$ARCHIVE" || err "failed to download $ARCHIVE_URL"

info "Downloading checksums.txt"
$DL "$CHECKSUMS_URL" > "$TMP/checksums.txt" || err "failed to download checksums"

info "Verifying SHA256..."
expected="$(grep " $ARCHIVE\$" "$TMP/checksums.txt" | awk '{print $1}')"
[ -n "$expected" ] || err "checksum entry not found for $ARCHIVE"

if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "$TMP/$ARCHIVE" | awk '{print $1}')"
elif command -v shasum >/dev/null 2>&1; then
    actual="$(shasum -a 256 "$TMP/$ARCHIVE" | awk '{print $1}')"
else
    err "need sha256sum or shasum"
fi

if [ "$expected" != "$actual" ]; then
    err "checksum mismatch (expected $expected, got $actual)"
fi
info "Checksum OK"

info "Extracting..."
tar -xzf "$TMP/$ARCHIVE" -C "$TMP"
[ -f "$TMP/papermap" ] || err "papermap binary missing from archive"
chmod +x "$TMP/papermap"

# Pick install prefix.
if [ -z "${PREFIX:-}" ]; then
    if [ "$(id -u)" = "0" ]; then
        PREFIX="/usr/local"
    elif [ -w "/usr/local/bin" ]; then
        PREFIX="/usr/local"
    else
        PREFIX="$HOME/.local"
    fi
fi

DEST="$PREFIX/bin"
mkdir -p "$DEST"

info "Installing to $DEST/papermap"
if mv "$TMP/papermap" "$DEST/papermap" 2>/dev/null; then
    :
else
    if command -v sudo >/dev/null 2>&1; then
        info "Elevating with sudo..."
        sudo mv "$TMP/papermap" "$DEST/papermap"
    else
        err "cannot write to $DEST and sudo not available"
    fi
fi

info "Installed: $($DEST/papermap --version 2>/dev/null || echo papermap)"

case ":$PATH:" in
    *":$DEST:"*) ;;
    *)
        printf '\nNote: %s is not on your PATH. Add it to your shell rc:\n' "$DEST"
        printf '    export PATH="%s:$PATH"\n\n' "$DEST"
        ;;
esac
