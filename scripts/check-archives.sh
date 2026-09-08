#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
set -euo pipefail
cd "$(dirname "$0")/.."

# The offline rebuild must stamp the same version, or the comparison below
# would report the stamp as a difference between the shipped binary and the
# source it was built from.
version="$(bash scripts/release-version.sh)"
ldflags="$(bash scripts/build-ldflags.sh "$version")"

check_dir=$(mktemp -d)
trap 'rm -rf "$check_dir"' EXIT
fail() { echo "check-archives.sh: $1" >&2; exit 1; }
(cd dist; shasum -a 256 -c SHA256SUMS)

for platform in darwin linux; do
  for arch in amd64 arm64; do
    name="agent-commons-$platform-$arch"
    tar -xzf "dist/$name.tar.gz" -C "$check_dir"
    archive="$check_dir/$name"
    node scripts/check-docs.mjs "$archive"
    cmp LICENSE "$archive/LICENSE"
    test -x "$archive/agent-commons"
    test -s "$archive/third-party-notices/go/LICENSE"
    test -s "$archive/third-party-notices/go/PATENTS"
    for module in bubbletea bubbles lipgloss; do
      test -s "$archive/third-party-notices/modules/charm.land/$module/v2/LICENSE"
    done
    for module in crypto net text; do
      test -s "$archive/third-party-notices/golang.org/x/$module/LICENSE"
    done
    # A per-platform archive is installed by `bundle install`, which knows the
    # destination and rewrites these manifests to the absolute path of the
    # binary it places. It refuses a command whose basename is not
    # agent-commons, so these copies must stay exactly as the source tree
    # carries them; the wrapper belongs to the plugin tarball alone.
    for manifest in mcp.json .mcp.json; do
      cmp "plugins/agent-commons/$manifest" "$archive/plugins/agent-commons/$manifest" ||
        fail "$name does not carry the source-tree $manifest"
    done
    mkdir "$archive/source"
    tar -xzf "$archive/source.tar.gz" -C "$archive/source"
    cmp LICENSE "$archive/source/LICENSE"
    (cd "$archive/source"; GOPROXY=off GOSUMDB=off CGO_ENABLED=0 GOOS="$platform" GOARCH="$arch" \
      go build -mod=vendor -trimpath -buildvcs=false -ldflags="$ldflags" \
      -o "$archive/rebuilt" ./cmd/agent-commons)
    cmp "$archive/agent-commons" "$archive/rebuilt"
    # Checks that must run the shipped binary, so only for this host.
    if [[ "$platform" == "$(go env GOOS)" && "$arch" == "$(go env GOARCH)" ]]; then
      # The shipped binary must report the version it was stamped with, from
      # the command line and from the runtime.status DTO alike. Release
      # archives are built with -buildvcs=false, so this stamp is the only
      # thing they can say about themselves.
      test "$("$archive/agent-commons" --version)" = "agent-commons $version"
      bash scripts/check-shipped-version.sh "$archive/agent-commons" "$version" "$check_dir"
      bash scripts/check-bundle-install.sh "$archive" "$check_dir"
    fi
  done
done

tar -xzf dist/agent-commons-plugin.tar.gz -C "$check_dir"
cmp LICENSE "$check_dir/agent-commons/LICENSE"
test -s "$check_dir/agent-commons/skills/agent-commons/SKILL.md"
cmp plugins/agent-commons/plugin.json "$check_dir/agent-commons/plugin.json"
# The plugin tarball is the one route that can never learn where it will be
# unpacked, so its MCP command must name the wrapper that resolves the binary at
# server start. The source tree keeps the bare command, because that is what
# `bundle install` validates and rewrites for the other route. Assert both forms
# separately, and assert that nothing else in the manifest was touched: undoing
# the intended rewrite must reproduce the source file byte for byte.
# The source-tree assertion below is not the first line of defense and does not
# claim to be. Converge the two routes by rewriting the source and the real
# `bundle install` above refuses it at internal/installation/manifest.go:81,
# before this section is reached. This is defense in depth: it names the cause
# instead of leaving a generic installer refusal, it covers a source rewrite
# made after packaging, and it survives changes to the bundle-install exercise.
test -x "$check_dir/agent-commons/scripts/connect-mcp.sh" ||
  fail "the plugin tarball ships no executable connect-mcp.sh for its MCP command"
for manifest in mcp.json .mcp.json; do
  source_manifest="plugins/agent-commons/$manifest"
  packaged="$check_dir/agent-commons/$manifest"
  grep -q '"command": "sh"' "$packaged" ||
    fail "the packaged $manifest does not run the wrapper: $(cat "$packaged")"
  grep -q '"args": \["\${CLAUDE_PLUGIN_ROOT}/scripts/connect-mcp.sh"\]' "$packaged" ||
    fail "the packaged $manifest does not name the wrapper: $(cat "$packaged")"
  grep -q '"command": "agent-commons"' "$source_manifest" ||
    fail "the source-tree $manifest no longer carries the bare command bundle install expects"
  grep -q '"args": \["connect-mcp"\]' "$source_manifest" ||
    fail "the source-tree $manifest no longer passes connect-mcp"
  sed -e 's|"command": "sh"|"command": "agent-commons"|' \
    -e 's|"args": \["\${CLAUDE_PLUGIN_ROOT}/scripts/connect-mcp.sh"\]|"args": ["connect-mcp"]|' \
    "$packaged" | cmp - "$source_manifest" ||
    fail "the packaged $manifest differs from the source beyond the MCP command"
done
node --test "$check_dir/agent-commons/pi/commons.test.mjs"
test -x "$check_dir/agent-commons/scripts/check-in.sh"
test -s "$check_dir/agent-commons/hooks/claude.json"
test -s "$check_dir/agent-commons/scripts/claude-session-start.sh"
