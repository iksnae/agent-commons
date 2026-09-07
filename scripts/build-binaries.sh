#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
mkdir -p dist
build_dir=$(mktemp -d)
trap 'rm -rf "$build_dir"' EXIT

for platform in darwin linux; do
  for arch in amd64 arm64; do
    name="agent-commons-${platform}-${arch}"
    mkdir -p "$build_dir/$name"
    CGO_ENABLED=0 GOOS="$platform" GOARCH="$arch" go build \
      -trimpath -buildvcs=false -ldflags='-s -w' \
      -o "$build_dir/$name/agent-commons" ./cmd/agent-commons
    cp README.md "$build_dir/$name/"
    tar -czf "dist/$name.tar.gz" -C "$build_dir" "$name"
  done
done

tar -czf dist/agent-commons-plugin.tar.gz -C plugins agent-commons
(
  cd dist
  shasum -a 256 agent-commons-darwin-amd64.tar.gz \
    agent-commons-darwin-arm64.tar.gz agent-commons-linux-amd64.tar.gz \
    agent-commons-linux-arm64.tar.gz agent-commons-plugin.tar.gz > SHA256SUMS
)
