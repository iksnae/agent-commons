#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
# Print the version to stamp into a build, for both the packaging build and the
# offline rebuild that is compared against it. Release archives are built with
# -buildvcs=false, so a stamped version is the only thing a released binary can
# say about itself.
#
# A tag names a release. Anything else is honestly "dev": this never invents a
# version number for an untagged tree.
set -euo pipefail

if [ -n "${AGENT_COMMONS_VERSION:-}" ]; then
  printf '%s\n' "$AGENT_COMMONS_VERSION"
  exit 0
fi
if version="$(git describe --tags --exact-match 2> /dev/null)"; then
  printf '%s\n' "$version"
  exit 0
fi
printf 'dev\n'
