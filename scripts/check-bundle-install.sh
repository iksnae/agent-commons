#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
set -euo pipefail

archive=$1
scratch=$2
installed="$scratch/installed"
"$archive/agent-commons" bundle install --from "$archive" --to "$installed"
"$installed/agent-commons" bundle verify --to "$installed"
"$installed/agent-commons" --help > /dev/null
"$installed/agent-commons" bundle remove --to "$installed" --confirm-stopped
test ! -e "$installed"
