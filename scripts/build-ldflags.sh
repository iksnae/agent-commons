#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
# Print the linker flags for a release build of ./cmd/agent-commons.
#
# The packaging build and the offline rebuild that is compared against it must
# pass byte-identical flags, or the comparison reports the flags themselves as a
# difference. Stating them once here is the same discipline the stamped variable
# follows: -X names a symbol the linker accepts without checking, so a stamp
# aimed at a stale path links cleanly and silently reports "dev". One string,
# two callers, no way to retarget one and forget the other.
set -euo pipefail
version=$1
printf -- '-s -w -X agentcommons/internal/core.Version=%s\n' "$version"
