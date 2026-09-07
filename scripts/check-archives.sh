#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
set -euo pipefail
cd "$(dirname "$0")/.."

check_dir=$(mktemp -d)
trap 'rm -rf "$check_dir"' EXIT
(cd dist; shasum -a 256 -c SHA256SUMS)

for platform in darwin linux; do
  for arch in amd64 arm64; do
    name="agent-commons-$platform-$arch"
    tar -xzf "dist/$name.tar.gz" -C "$check_dir"
    archive="$check_dir/$name"
    cmp LICENSE "$archive/LICENSE"
    test -x "$archive/agent-commons"
    test -s "$archive/third-party-notices/go/LICENSE"
    test -s "$archive/third-party-notices/go/PATENTS"
    for module in crypto net text; do
      test -s "$archive/third-party-notices/golang.org/x/$module/LICENSE"
    done
    mkdir "$archive/source"
    tar -xzf "$archive/source.tar.gz" -C "$archive/source"
    cmp LICENSE "$archive/source/LICENSE"
    (cd "$archive/source"; CGO_ENABLED=0 GOOS="$platform" GOARCH="$arch" \
      go build -trimpath -buildvcs=false -ldflags='-s -w' \
      -o "$archive/rebuilt" ./cmd/agent-commons)
    cmp "$archive/agent-commons" "$archive/rebuilt"
  done
done

tar -xzf dist/agent-commons-plugin.tar.gz -C "$check_dir"
cmp LICENSE "$check_dir/agent-commons/LICENSE"
test -s "$check_dir/agent-commons/skills/agent-commons/SKILL.md"
test -x "$check_dir/agent-commons/scripts/check-in.sh"
