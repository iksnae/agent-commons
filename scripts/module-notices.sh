#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
set -euo pipefail

# Run inside the source snapshot after go mod vendor. Go vendors dependency
# license files alongside the exact packages included in the source bundle.
destination=$1
while IFS= read -r -d '' notice; do
  case "$(basename "$notice")" in
    LICENSE*|LICENCE*|COPYING*|NOTICE*|PATENTS*|AUTHORS*)
      relative=${notice#vendor/}
      mkdir -p "$destination/modules/$(dirname "$relative")"
      cp "$notice" "$destination/modules/$relative"
      ;;
  esac
done < <(find vendor -type f -print0)
