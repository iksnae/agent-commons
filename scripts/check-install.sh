#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
# Exercise the installer offline: syntax, a verified local archive, and the
# refusal paths. No network, no writes outside the scratch directory.
set -euo pipefail
cd "$(dirname "$0")/.."

bash -n scripts/install.sh

check_dir=$(mktemp -d)
trap 'rm -rf "$check_dir"' EXIT

case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  *) echo "check-install.sh does not run on $(uname -s)" >&2; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) echo "check-install.sh does not run on $(uname -m)" >&2; exit 1 ;;
esac
name="agent-commons-$os-$arch"

# A stand-in archive with the distributable layout. Building real binaries for
# this check would repeat `just package`; the installer only reads the layout.
mkdir -p "$check_dir/stage/$name"
printf '#!/bin/sh\necho stand-in agent-commons\n' > "$check_dir/stage/$name/agent-commons"
chmod 755 "$check_dir/stage/$name/agent-commons"
mkdir "$check_dir/release"
tar -czf "$check_dir/release/$name.tar.gz" -C "$check_dir/stage" "$name"
(cd "$check_dir/release"; shasum -a 256 "$name.tar.gz" > SHA256SUMS)

home="$check_dir/home"
mkdir "$home"

# A verified local archive installs the binary and nothing else.
dest="$check_dir/bin"
HOME="$home" PATH="/usr/bin:/bin:/usr/sbin:/sbin" \
  bash scripts/install.sh --archive "$check_dir/release/$name.tar.gz" --to "$dest" \
  > "$check_dir/install.log" 2>&1
test -x "$dest/agent-commons"
grep -q "checksum verified" "$check_dir/install.log"
grep -q "$dest/agent-commons" "$check_dir/install.log"
grep -q 'export PATH=' "$check_dir/install.log"
# The destination holds the binary only; no profile, PATH or home state written.
test "$(ls -A "$dest")" = "agent-commons"
test -z "$(ls -A "$home")"

# A damaged archive is refused: non-zero exit, no install, operator file kept.
mkdir "$check_dir/damaged"
cp "$check_dir/release/$name.tar.gz" "$check_dir/release/SHA256SUMS" "$check_dir/damaged/"
printf 'damage' >> "$check_dir/damaged/$name.tar.gz"
bad_dest="$check_dir/bin-damaged"
if HOME="$home" bash scripts/install.sh --archive "$check_dir/damaged/$name.tar.gz" \
  --to "$bad_dest" > "$check_dir/damaged.log" 2>&1; then
  echo "installer accepted a damaged archive" >&2
  cat "$check_dir/damaged.log" >&2
  exit 1
fi
grep -q 'checksum mismatch' "$check_dir/damaged.log"
test ! -e "$bad_dest/agent-commons"
test -f "$check_dir/damaged/$name.tar.gz"

# An archive with no SHA256SUMS beside it announces the gap and still installs.
# The operator named a file on their own disk; the script must not claim it checked it.
mkdir "$check_dir/unchecked"
cp "$check_dir/release/$name.tar.gz" "$check_dir/unchecked/"
plain_dest="$check_dir/bin-unchecked"
HOME="$home" bash scripts/install.sh --archive "$check_dir/unchecked/$name.tar.gz" \
  --to "$plain_dest" > "$check_dir/unchecked.log" 2>&1
test -x "$plain_dest/agent-commons"
grep -q 'installing the file you supplied unchecked' "$check_dir/unchecked.log"
if grep -q 'checksum verified' "$check_dir/unchecked.log"; then
  echo "installer claimed a checksum it never had" >&2
  exit 1
fi

# An archive whose members escape their own directory is refused before unpacking.
mkdir -p "$check_dir/escape/$name"
cp "$check_dir/stage/$name/agent-commons" "$check_dir/escape/$name/agent-commons"
echo escaped > "$check_dir/escape/marker"
(cd "$check_dir/escape/$name" && tar -czPf "$check_dir/escape/$name.tar.gz" agent-commons ../marker)
# Assert the mutant is really malformed, so a sanitizing tar fails loudly here
# rather than letting the guard's test pass without exercising it.
tar -tzf "$check_dir/escape/$name.tar.gz" | grep -q '^\.\./marker$'
esc_dest="$check_dir/bin-escape"
if HOME="$home" bash scripts/install.sh --archive "$check_dir/escape/$name.tar.gz" \
  --to "$esc_dest" > "$check_dir/escape.log" 2>&1; then
  echo "installer unpacked an archive with escaping members" >&2
  exit 1
fi
grep -q 'outside its own directory' "$check_dir/escape.log"
test ! -e "$esc_dest/agent-commons"

# An unreadable archive path fails loudly instead of installing nothing quietly.
if HOME="$home" bash scripts/install.sh --archive "$check_dir/absent.tar.gz" --to "$check_dir/bin-absent" \
  > "$check_dir/absent.log" 2>&1; then
  echo "installer accepted a missing archive" >&2
  exit 1
fi
grep -q 'error:' "$check_dir/absent.log"
test ! -e "$check_dir/bin-absent"

echo "Installer verified: verified archive, checksum mismatch, unchecked archive, escaping members, missing archive."
