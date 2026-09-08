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
STAGED=""
TMP="$(mktemp -d)"
cleanup() {
  rm -rf "$TMP"
  if [ -n "$STAGED" ]; then rm -f "$STAGED"; fi
}
trap cleanup EXIT
# What keeps an interrupted install from leaving a staged file behind is the
# EXIT cleanup above together with `|| error` on every step below, not these
# signal traps. HUP and TERM do reach cleanup by exiting through it. INT does
# not reliably abort: bash discards a pending SIGINT trap when the foreground
# child did not itself die of SIGINT, so a Ctrl-C can leave the install to run
# to completion. Do not treat the INT line as an interrupt guarantee.
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

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
  command -v curl > /dev/null 2>&1 \
    || error "curl is required to download a release. Install curl, or pass a local archive with --archive PATH."
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
# An archive may only write inside its own directory. On the unchecked --archive
# route nothing else constrains its members.
if tar -tzf "$TMP/$NAME.tar.gz" | grep -Eq '^/|(^|/)\.\.(/|$)'; then
  error "Archive contains paths outside its own directory. Refusing to unpack it."
fi
tar -xzf "$TMP/$NAME.tar.gz" -C "$TMP/unpacked"
BINARY="$TMP/unpacked/$NAME/agent-commons"
[ -f "$BINARY" ] || error "Archive does not contain $NAME/agent-commons."

mkdir -p "$DEST" || error "Cannot create the install directory $DEST. Nothing was installed."
DEST="$(cd "$DEST" && pwd)" || error "Cannot enter the install directory $DEST. Nothing was installed."
TARGET="$DEST/agent-commons"
# A rename onto a directory moves the staged file inside it and reports success,
# so refuse anything that is not an ordinary file. Copying did the same thing
# just as quietly, so this closes a hole that was already here rather than one
# the rename opened.
#
# `-f` follows symlinks, so a symlink to a regular file passes this guard and
# the rename replaces the link itself, leaving whatever it pointed at untouched.
# Copying wrote through the link instead. If you point this path at a versioned
# binary, an install leaves a real file here and your link is gone.
if [ -e "$TARGET" ] && [ ! -f "$TARGET" ]; then
  error "$TARGET exists and is not a regular file. Refusing to install over it."
fi

# Replace the installed file by renaming a staged copy over it, never by
# writing into it. A process already running from $TARGET keeps executing the
# file it was started from; overwriting that file in place invalidates its
# image, and macOS kills the process with SIGKILL (exit 137). Linux refuses the
# write with ETXTBSY instead, so the same mistake fails the install rather than
# the service. Neither outcome is acceptable, and upgrading over a running
# install is the ordinary case, so this is the ordinary path.
# cmd/agent-commons/update.go holds the same property for `agent-commons update`.
#
# The staged copy must live in $DEST. `mv` across filesystems is a copy and an
# unlink rather than a rename, so staging in $TMP — routinely a different
# filesystem from ~/.local/bin — would reintroduce exactly this defect while
# looking like a fix. The template below keeps the staged file in $DEST, and
# the check after it fails loudly if a later edit moves it out.
STAGED="$(mktemp "$DEST/.agent-commons-install.XXXXXX")" \
  || error "Cannot write into $DEST. Nothing was installed; any existing $TARGET is unchanged."
case "$STAGED" in
  "$DEST"/*) ;;
  *) error "Staged $STAGED outside $DEST, where a rename onto $TARGET would not be atomic. Nothing was installed." ;;
esac

cp "$BINARY" "$STAGED" \
  || error "Cannot write $STAGED. Nothing was installed; any existing $TARGET is unchanged."
chmod 755 "$STAGED" \
  || error "Cannot set permissions on $STAGED. Nothing was installed; any existing $TARGET is unchanged."
mv -f "$STAGED" "$TARGET" \
  || error "Cannot replace $TARGET. Nothing was installed; the existing file is unchanged."
STAGED=""
info "Installed agent-commons to $TARGET"

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
