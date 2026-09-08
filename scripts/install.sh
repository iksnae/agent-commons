#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
# Agent Commons — install the CLI for one operator, into one directory.
# Usage: curl -fsSL https://raw.githubusercontent.com/iksnae/agent-commons/main/scripts/install.sh | bash
# It copies a binary. It does not edit your shell profile, write global config,
# start a service, enroll a role, or use sudo.
set -euo pipefail

REPO="iksnae/agent-commons"
DEST="$HOME/.local/bin"
ARCHIVE=""
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

info()  { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
error() { printf '\033[1;31merror:\033[0m %s\n' "$*" >&2; exit 1; }

while [ $# -gt 0 ]; do
  case "$1" in
    --to) [ $# -ge 2 ] || error "--to needs a directory"; DEST="$2"; shift 2 ;;
    --archive) [ $# -ge 2 ] || error "--archive needs a file"; ARCHIVE="$2"; shift 2 ;;
    -h|--help)
      printf 'install.sh [--to DIR] [--archive PATH]\n'
      printf '  --to DIR       install directory (default: ~/.local/bin)\n'
      printf '  --archive PATH install a local archive instead of downloading\n'
      exit 0 ;;
    *) error "Unknown option: $1" ;;
  esac
done

case "$(uname -s)" in
  Darwin) OS="darwin" ;;
  Linux) OS="linux" ;;
  *) error "Unsupported platform: $(uname -s). Published archives cover macOS and Linux only." ;;
esac
case "$(uname -m)" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) error "Unsupported architecture: $(uname -m). Published archives cover amd64 and arm64 only." ;;
esac
NAME="agent-commons-$OS-$ARCH"

if [ -n "$ARCHIVE" ]; then
  [ -f "$ARCHIVE" ] || error "No such archive: $ARCHIVE"
  info "Using local archive $ARCHIVE"
  cp "$ARCHIVE" "$TMP/$NAME.tar.gz"
  SUMS="$(cd "$(dirname "$ARCHIVE")" && pwd)/SHA256SUMS"
  if [ -f "$SUMS" ]; then
    cp "$SUMS" "$TMP/SHA256SUMS"
  else
    info "No SHA256SUMS beside the archive; installing the file you supplied unchecked."
  fi
else
  info "Looking up the latest release of $REPO …"
  TAG="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | grep -o '"tag_name"[^,]*' | head -1 | cut -d'"' -f4 || true)"
  [ -n "$TAG" ] || error "No published release found for $REPO. Build an archive yourself and pass it with --archive PATH; see INSTALL.md."
  BASE="https://github.com/$REPO/releases/download/$TAG"
  info "Downloading $NAME.tar.gz ($TAG) …"
  curl -fsSL "$BASE/$NAME.tar.gz" -o "$TMP/$NAME.tar.gz" \
    || error "Release $TAG has no $NAME.tar.gz. Build an archive yourself and pass it with --archive PATH; see INSTALL.md."
  curl -fsSL "$BASE/SHA256SUMS" -o "$TMP/SHA256SUMS" \
    || error "Release $TAG has no SHA256SUMS, so the download cannot be checked. Refusing to install."
  [ -s "$TMP/$NAME.tar.gz" ] || error "Downloaded $NAME.tar.gz is empty. Refusing to install."
fi

# A checksum beside an archive detects transfer damage, not who published it.
# Agent Commons has no release signing yet; this check does not prove authorship.
if [ -f "$TMP/SHA256SUMS" ]; then
  expected="$(awk -v file="$NAME.tar.gz" '$2 == file || $2 == "*" file {print $1}' "$TMP/SHA256SUMS" | head -1)"
  [ -n "$expected" ] || error "SHA256SUMS has no entry for $NAME.tar.gz. Refusing to install."
  if command -v shasum > /dev/null 2>&1; then
    actual="$(shasum -a 256 "$TMP/$NAME.tar.gz" | cut -d' ' -f1)"
  elif command -v sha256sum > /dev/null 2>&1; then
    actual="$(sha256sum "$TMP/$NAME.tar.gz" | cut -d' ' -f1)"
  else
    error "Neither shasum nor sha256sum is available, so the archive cannot be checked. Refusing to install."
  fi
  if [ "$expected" != "$actual" ]; then
    printf '  expected %s\n  actual   %s\n' "$expected" "$actual" >&2
    rm -f "$TMP/$NAME.tar.gz"
    error "checksum mismatch for $NAME.tar.gz. The copy was discarded and nothing was installed."
  fi
  info "SHA256 checksum verified (damage in transfer only; not a publisher signature)."
fi

mkdir -p "$TMP/unpacked"
tar -xzf "$TMP/$NAME.tar.gz" -C "$TMP/unpacked"
BINARY="$TMP/unpacked/$NAME/agent-commons"
[ -f "$BINARY" ] || error "Archive does not contain $NAME/agent-commons."

mkdir -p "$DEST"
cp "$BINARY" "$DEST/agent-commons"
chmod 755 "$DEST/agent-commons"
info "Installed agent-commons to $DEST/agent-commons"

case ":${PATH-}:" in
  *":$DEST:"*) ;;
  *)
    case "${SHELL##*/}" in
      zsh) PROFILE="~/.zshrc" ;;
      bash) PROFILE="~/.bashrc" ;;
      *) PROFILE="~/.profile" ;;
    esac
    info "$DEST is not on your PATH. Add this line to $PROFILE yourself; this installer does not edit it:"
    printf '  export PATH="%s:$PATH"\n' "$DEST"
    ;;
esac

info "Next: run 'agent-commons init' in a project. See README.md."
