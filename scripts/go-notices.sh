#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
set -euo pipefail

destination=$1
go_root=$(go env GOROOT)
license="$go_root/LICENSE"
# Homebrew keeps this notice one directory above GOROOT.
if [[ ! -f "$license" ]]; then
  license="$(dirname "$go_root")/LICENSE"
fi
mkdir -p "$destination/go"
cp "$license" "$destination/go/LICENSE"
cp "$go_root/PATENTS" "$destination/go/PATENTS"
# Include Go's bundled library notices; never replace their terms with MPL.
for notice in "$go_root"/src/vendor/golang.org/x/*/LICENSE \
              "$go_root"/src/vendor/golang.org/x/*/PATENTS; do
  [[ -f "$notice" ]] || continue
  relative=${notice#"$go_root/src/vendor/"}
  mkdir -p "$destination/$(dirname "$relative")"
  cp "$notice" "$destination/$relative"
done
