#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
# Assert that a shipped binary reports the version it was stamped with from
# BOTH surfaces that answer "which release is running": the command line and
# the runtime.status DTO that agents read. Reading the -ldflags line proves
# nothing — a stamp aimed at a variable no longer read still links cleanly and
# silently reports "dev". Only running the extracted binary settles it.
set -euo pipefail

binary=$1
version=$2
scratch=$3

printed="$("$binary" version)"
if [ "$printed" != "agent-commons $version" ]; then
  echo "check-shipped-version.sh: \`version\` printed '$printed', want 'agent-commons $version'" >&2
  exit 1
fi

# macOS caps unix socket paths near 104 bytes, so the state directory is short
# rather than nested under the caller's scratch tree.
state=$(mktemp -d /tmp/acship.XXXXXX)
cleanup() {
  if [ -n "${service_pid:-}" ]; then
    kill "$service_pid" 2> /dev/null || true
    wait "$service_pid" 2> /dev/null || true
  fi
  rm -rf "$state"
}
trap cleanup EXIT

"$binary" serve --state "$state" > "$scratch/serve.log" 2>&1 &
service_pid=$!
for _ in $(seq 1 200); do
  if [ -S "$state/service.sock" ] && [ -f "$state/operator.token" ]; then break; fi
  sleep 0.1
done
if [ ! -S "$state/service.sock" ] || [ ! -f "$state/operator.token" ]; then
  echo "check-shipped-version.sh: service did not become reachable: $(cat "$scratch/serve.log")" >&2
  exit 1
fi

status="$("$binary" call --state "$state" --token-file "$state/operator.token" runtime.status '{}')"
reported="$(printf '%s' "$status" | node -e 'let i="";process.stdin.on("data",d=>i+=d).on("end",()=>process.stdout.write(String(JSON.parse(i).build.version)))')"
if [ "$reported" != "$version" ]; then
  echo "check-shipped-version.sh: runtime.status reported version '$reported', want '$version'" >&2
  exit 1
fi
