#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
# Exercise the installer offline: syntax, a verified local archive, and the
# refusal paths. Its fixtures are all local and it installs only into the
# scratch directory. It is not hermetic beyond that: the running-binary case
# compiles a Go stand-in, which writes to GOCACHE outside the scratch directory,
# and under GOTOOLCHAIN=auto a toolchain older than that stand-in's `go 1.26`
# would fetch one over the network.
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

# Installing over a binary that is CURRENTLY RUNNING from the destination path.
# Every case above installs into an empty directory, which is why they all
# passed while the real upgrade path was broken: writing onto a running
# executable in place invalidates its image, and macOS kills the process with
# SIGKILL (exit 137) while Linux refuses the write with ETXTBSY. The install
# must replace the directory entry by renaming a staged file over it, so a
# process already running keeps the file it started from.
# cmd/agent-commons/update.go holds the same property for `update`.
#
# The three assertions below are deliberately independent, and which one fires
# first is platform-dependent: the held handle and the inode both detect an
# in-place write directly, while the surviving process detects the consequence.
#
# The previous install has to be a compiled binary, and neither shortcut works:
# a shell script is re-read from its path, so a running one picks up the new
# file even when the install is atomic, and a copy of an Apple platform binary
# such as /bin/sleep is killed on sight whatever the installer did. Both report
# on macOS rather than on this script. So build one; it needs no dependencies
# and `just check` already requires the Go toolchain.
mkdir -p "$check_dir/previous-src"
cat > "$check_dir/previous-src/go.mod" <<'MOD'
module previousinstall

go 1.26
MOD
cat > "$check_dir/previous-src/main.go" <<'GO'
// The binary a previous install left at the destination, still running.
package main

import "time"

func main() { time.Sleep(10 * time.Minute) }
GO
(cd "$check_dir/previous-src" && go build -o "$check_dir/previous-install" .)

live_dest="$check_dir/bin-live"
mkdir -p "$live_dest"
cp "$check_dir/previous-install" "$live_dest/agent-commons"
chmod 755 "$live_dest/agent-commons"
"$live_dest/agent-commons" &
live_pid=$!
# Keep job control from reporting this script's own cleanup kill as a failure.
disown "$live_pid" 2> /dev/null || true
trap 'kill "$live_pid" 2> /dev/null || true; rm -rf "$check_dir"' EXIT
# Hold the file the running process was started from, the same way
# TestUpdateRenamesOverTheTargetRatherThanWritingIntoIt holds its handle.
exec 9< "$live_dest/agent-commons"
before_inode="$(ls -i "$live_dest/agent-commons" | awk '{print $1}')"

HOME="$home" PATH="/usr/bin:/bin:/usr/sbin:/sbin" \
  bash scripts/install.sh --archive "$check_dir/release/$name.tar.gz" --to "$live_dest" \
  > "$check_dir/live.log" 2>&1

# The newly installed file is at the destination and executes.
test "$("$live_dest/agent-commons")" = "stand-in agent-commons"
# The held handle still reads the file the running process started from. An
# install that writes into the existing file reads the installed bytes here.
if ! cmp -s /dev/fd/9 "$check_dir/previous-install"; then
  echo "install wrote into the file a running process was started from" >&2
  exit 1
fi
exec 9<&-
# The path carries a different file than before, not the same file rewritten.
after_inode="$(ls -i "$live_dest/agent-commons" | awk '{print $1}')"
if [ "$before_inode" = "$after_inode" ]; then
  echo "install reused the inode at $live_dest/agent-commons instead of renaming over it" >&2
  exit 1
fi
# The process started before the install is still running, not a killed corpse.
live_state="$(ps -o state= -p "$live_pid" 2> /dev/null | tr -d ' ')"
case "$live_state" in
  ""|Z*)
    echo "the process running from the destination path did not survive the install ($live_state)" >&2
    exit 1 ;;
esac
kill "$live_pid" 2> /dev/null || true
# The destination holds the installed binary only; no staged file was left.
test "$(ls -A "$live_dest")" = "agent-commons"

# A staged file abandoned by an interrupted earlier run neither blocks the next
# install nor is mistaken for one.
stale_dest="$check_dir/bin-stale"
mkdir -p "$stale_dest"
printf 'half a binary' > "$stale_dest/.agent-commons-install.LEFTOVER"
HOME="$home" bash scripts/install.sh --archive "$check_dir/release/$name.tar.gz" \
  --to "$stale_dest" > "$check_dir/stale.log" 2>&1
test "$("$stale_dest/agent-commons")" = "stand-in agent-commons"
test "$(ls -A "$stale_dest" | grep -c '^\.agent-commons-install\.')" = "1"
test "$(cat "$stale_dest/.agent-commons-install.LEFTOVER")" = "half a binary"

# A destination that cannot be staged into fails before anything is replaced:
# the operator's existing binary survives intact and no partial file is left.
locked_dest="$check_dir/bin-locked"
mkdir -p "$locked_dest"
printf '#!/bin/sh\necho operator binary\n' > "$locked_dest/agent-commons"
chmod 755 "$locked_dest/agent-commons"
chmod 555 "$locked_dest"
if HOME="$home" bash scripts/install.sh --archive "$check_dir/release/$name.tar.gz" \
  --to "$locked_dest" > "$check_dir/locked.log" 2>&1; then
  chmod 755 "$locked_dest"
  echo "installer reported success on a destination it cannot write" >&2
  cat "$check_dir/locked.log" >&2
  exit 1
fi
chmod 755 "$locked_dest"
grep -q 'error:' "$check_dir/locked.log"
test "$("$locked_dest/agent-commons")" = "operator binary"
test "$(ls -A "$locked_dest")" = "agent-commons"

echo "Installer verified: verified archive, checksum mismatch, unchecked archive, escaping members, missing archive, install over a running binary, stale staged file, unwritable destination."
