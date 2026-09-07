#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
set -euo pipefail

cd "$(dirname "$0")/.."
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
      -mod=vendor -trimpath -buildvcs=false -ldflags='-s -w' \
      -o "$build_dir/$name/agent-commons" ./cmd/agent-commons)
    cp "$build_dir/source/README.md" "$build_dir/source/LICENSE" \
      "$build_dir/source/LICENSING.md" "$build_dir/source/INSTALL.md" "$build_dir/source/SERVICE.md" "$build_dir/source.tar.gz" "$build_dir/$name/"
    cp -R "$build_dir/notices" "$build_dir/$name/third-party-notices"
    tar -czf "$build_dir/artifacts/$name.tar.gz" -C "$build_dir" "$name"
  done
done

tar -czf "$build_dir/artifacts/agent-commons-plugin.tar.gz" \
  -C "$build_dir/source/plugins" agent-commons
(
  cd "$build_dir/artifacts"
  shasum -a 256 agent-commons-darwin-amd64.tar.gz \
    agent-commons-darwin-arm64.tar.gz agent-commons-linux-amd64.tar.gz \
    agent-commons-linux-arm64.tar.gz agent-commons-plugin.tar.gz > SHA256SUMS
)
cp "$build_dir/artifacts/"* dist/
