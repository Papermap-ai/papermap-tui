#!/usr/bin/env sh
# Papermap TUI installer.
#
# Downloads the latest papermap-tui release for your OS/arch from GitHub
# Releases, verifies its SHA256 checksum, and installs the binary to
# $PREFIX/bin (default: /usr/local or ~/.local).
#
# Usage (public repo):
#   curl -fsSL https://raw.githubusercontent.com/Papermap-ai/papermap-tui/main/install.sh | sh
#
# Usage (private repo, requires a GitHub token with repo:read access):
#   GH_TOKEN=ghp_xxx sh -c "$(curl -fsSL \
#       -H 'Authorization: Bearer ghp_xxx' \
#       https://raw.githubusercontent.com/Papermap-ai/papermap-tui/main/install.sh)"
#
# Environment overrides:
#   PREFIX        Installation root (default /usr/local or ~/.local)
#   VERSION       Release tag to install (default: latest)
#   REPO          GitHub repo (default Papermap-ai/papermap-tui)
#   GH_TOKEN      GitHub token for private-repo access (alias: GITHUB_TOKEN)
#   GITHUB_TOKEN  Same as GH_TOKEN (whichever is set is used)
#
# TODO(public-repo): when this repo becomes public, delete the
# GH_TOKEN / GITHUB_TOKEN handling below (search for "TOKEN" in this file)
# and revert to plain anonymous downloads. The token plumbing is only
# needed while the repo is private and is a footgun otherwise (tokens
# get pasted into shared docs, leak via process lists with `set -x`,
# etc.). Remove the related private-repo section from README.md too.

set -eu

REPO="${REPO:-Papermap-ai/papermap-tui}"
VERSION="${VERSION:-latest}"

# Resolve auth token (GH_TOKEN preferred, GITHUB_TOKEN as fallback).
TOKEN="${GH_TOKEN:-${GITHUB_TOKEN:-}}"

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

# Pick downloader. We wrap curl/wget in helpers so the auth header is
# attached uniformly and never echoed.
DL_TOOL=""
if command -v curl >/dev/null 2>&1; then
    DL_TOOL="curl"
elif command -v wget >/dev/null 2>&1; then
    DL_TOOL="wget"
else
    err "need curl or wget"
fi

# Fetch a URL to stdout. Adds the auth header when TOKEN is set.
# Args: URL [accept-header]
dl_to_stdout() {
    _url="$1"
    _accept="${2:-}"
    if [ "$DL_TOOL" = "curl" ]; then
        set -- -fsSL
        [ -n "$_accept" ] && set -- "$@" -H "Accept: $_accept"
        [ -n "$TOKEN" ] && set -- "$@" -H "Authorization: Bearer $TOKEN"
        curl "$@" "$_url"
    else
        set -- -qO-
        [ -n "$_accept" ] && set -- "$@" --header="Accept: $_accept"
        [ -n "$TOKEN" ] && set -- "$@" --header="Authorization: Bearer $TOKEN"
        wget "$@" "$_url"
    fi
}

# Fetch a URL to a file path. Adds the auth header when TOKEN is set.
# Args: URL DEST [accept-header]
dl_to_file() {
    _url="$1"
    _dest="$2"
    _accept="${3:-}"
    if [ "$DL_TOOL" = "curl" ]; then
        set -- -fsSL -o "$_dest"
        [ -n "$_accept" ] && set -- "$@" -H "Accept: $_accept"
        [ -n "$TOKEN" ] && set -- "$@" -H "Authorization: Bearer $TOKEN"
        curl "$@" "$_url"
    else
        set -- -qO "$_dest"
        [ -n "$_accept" ] && set -- "$@" --header="Accept: $_accept"
        [ -n "$TOKEN" ] && set -- "$@" --header="Authorization: Bearer $TOKEN"
        wget "$@" "$_url"
    fi
}

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

API_BASE="https://api.github.com/repos/$REPO"

# Resolve version tag.
if [ "$VERSION" = "latest" ]; then
    info "Resolving latest release..."
    TAG="$(dl_to_stdout "$API_BASE/releases/latest" "application/vnd.github+json" \
        | grep '"tag_name":' | head -n1 \
        | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
    [ -n "$TAG" ] || err "failed to resolve latest release tag (is the repo private and \$GH_TOKEN unset?)"
else
    TAG="$VERSION"
fi

VER="${TAG#v}"
ARCHIVE="papermap_${VER}_${OS}_${ARCH}.tar.gz"

info "Installing papermap $TAG ($OS/$ARCH)"

TMP="$(mktemp -d 2>/dev/null || mktemp -d -t papermap)"
trap 'rm -rf "$TMP"' EXIT INT TERM

# Choose download URLs based on whether we have a token.
# Anonymous: use the public releases/download path (works for public repos).
# Authenticated: use the API assets endpoint, which requires looking up
# each asset id first since it can't be guessed from the file name.
if [ -z "$TOKEN" ]; then
    BASE_URL="https://github.com/$REPO/releases/download/$TAG"

    info "Downloading $ARCHIVE"
    dl_to_file "$BASE_URL/$ARCHIVE" "$TMP/$ARCHIVE" \
        || err "failed to download $ARCHIVE"

    info "Downloading checksums.txt"
    dl_to_file "$BASE_URL/checksums.txt" "$TMP/checksums.txt" \
        || err "failed to download checksums"
else
    info "Resolving release assets..."
    release_json="$TMP/release.json"
    dl_to_file "$API_BASE/releases/tags/$TAG" "$release_json" "application/vnd.github+json" \
        || err "failed to fetch release metadata for $TAG"

    # Extract asset id by name. Avoids needing jq.
    asset_id_for() {
        _name="$1"
        # Walk the JSON: look for "name": "<name>" then back up to find the
        # nearest preceding "id": <number>. Newline-normalize first.
        tr ',' '\n' < "$release_json" \
            | awk -v want="\"$_name\"" '
                /"id":/ { match($0, /[0-9]+/); id = substr($0, RSTART, RLENGTH) }
                $0 ~ "\"name\": *" want { print id; exit }
            '
    }

    archive_id="$(asset_id_for "$ARCHIVE")"
    [ -n "$archive_id" ] || err "asset $ARCHIVE not found in release $TAG"
    sums_id="$(asset_id_for "checksums.txt")"
    [ -n "$sums_id" ] || err "checksums.txt not found in release $TAG"

    info "Downloading $ARCHIVE"
    dl_to_file "$API_BASE/releases/assets/$archive_id" "$TMP/$ARCHIVE" "application/octet-stream" \
        || err "failed to download $ARCHIVE"

    info "Downloading checksums.txt"
    dl_to_file "$API_BASE/releases/assets/$sums_id" "$TMP/checksums.txt" "application/octet-stream" \
        || err "failed to download checksums"
fi

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
