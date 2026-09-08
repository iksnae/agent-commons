#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
set -euo pipefail

cd "$(dirname "$0")/.."
version="$(bash scripts/release-version.sh)"
ldflags="$(bash scripts/build-ldflags.sh "$version")"
echo "Stamping build version: $version"
mkdir -p dist
build_dir=$(mktemp -d)
trap 'rm -rf "$build_dir"' EXIT

# Snapshot tracked working-tree files once; build and ship that same source.
# Stage new source files before packaging. Local state and untracked files stay out.
git ls-files -z | tar --null -T - -czf "$build_dir/source.tar.gz"
mkdir "$build_dir/source" "$build_dir/artifacts"
tar -xzf "$build_dir/source.tar.gz" -C "$build_dir/source"
bash scripts/go-notices.sh "$build_dir/notices"
(cd "$build_dir/source"; go mod vendor)
(cd "$build_dir/source"; bash scripts/module-notices.sh "$build_dir/notices")
# Ship vendored dependency source and notices; extracted bundles rebuild offline.
tar -czf "$build_dir/source.tar.gz" -C "$build_dir/source" .

for platform in darwin linux; do
  for arch in amd64 arm64; do
    name="agent-commons-${platform}-${arch}"
    mkdir -p "$build_dir/$name"
    (cd "$build_dir/source"; CGO_ENABLED=0 GOOS="$platform" GOARCH="$arch" go build \
      -mod=vendor -trimpath -buildvcs=false -ldflags="$ldflags" \
      -o "$build_dir/$name/agent-commons" ./cmd/agent-commons)
    cp "$build_dir/source/README.md" "$build_dir/source/LICENSE" \
      "$build_dir/source/LICENSING.md" "$build_dir/source/INSTALL.md" "$build_dir/source/SERVICE.md" \
      "$build_dir/source/WAKE-MAINTENANCE.md" "$build_dir/source/HARNESS-SUPPORT.md" "$build_dir/source.tar.gz" "$build_dir/$name/"
    cp -R "$build_dir/notices" "$build_dir/$name/third-party-notices"
    cp "$build_dir/source/"*.md "$build_dir/$name/"
    cp -R "$build_dir/source/docs" "$build_dir/source/gates" \
      "$build_dir/source/integrations" "$build_dir/source/plugins" "$build_dir/$name/"
    tar -czf "$build_dir/artifacts/$name.tar.gz" -C "$build_dir" "$name"
  done
done

# The plugin tarball is unpacked wherever its user chooses. The per-platform
# archives ship the source-tree manifests verbatim because `bundle install`
# knows the destination and rewrites the command to the absolute path of the
# binary it places; this tarball never learns that path, so its packaged copy
# points at the wrapper instead, which resolves the binary at server start with
# the same tiers the hook scripts use. Only this copy is rewritten: the source
# tree keeps the bare command that `bundle install` validates.
packaged="$build_dir/plugin/agent-commons"
mkdir -p "$build_dir/plugin"
cp -R "$build_dir/source/plugins/agent-commons" "$packaged"
for manifest in "$packaged/mcp.json" "$packaged/.mcp.json"; do
  sed -e 's|"command": "agent-commons"|"command": "sh"|' \
    -e 's|"args": \["connect-mcp"\]|"args": ["${CLAUDE_PLUGIN_ROOT}/scripts/connect-mcp.sh"]|' \
    "$manifest" > "$manifest.wrapped"
  # A silent no-op substitution would ship the defect this rewrite exists to
  # fix, so confirm both halves landed and no bare command survived.
  if ! grep -q '"command": "sh"' "$manifest.wrapped" ||
    ! grep -q '"args": \["\${CLAUDE_PLUGIN_ROOT}/scripts/connect-mcp.sh"\]' "$manifest.wrapped" ||
    grep -q '"command": "agent-commons"' "$manifest.wrapped"; then
    echo "build-binaries.sh: failed to point $manifest at the connect-mcp wrapper" >&2
    exit 1
  fi
  mv "$manifest.wrapped" "$manifest"
done
test -x "$packaged/scripts/connect-mcp.sh"
tar -czf "$build_dir/artifacts/agent-commons-plugin.tar.gz" \
  -C "$build_dir/plugin" agent-commons
(
  cd "$build_dir/artifacts"
  shasum -a 256 agent-commons-darwin-amd64.tar.gz \
    agent-commons-darwin-arm64.tar.gz agent-commons-linux-amd64.tar.gz \
    agent-commons-linux-arm64.tar.gz agent-commons-plugin.tar.gz > SHA256SUMS
)
cp "$build_dir/artifacts/"* dist/
