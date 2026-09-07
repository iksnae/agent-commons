#!/bin/sh
# SPDX-License-Identifier: MPL-2.0
set -eu
: "${AGENT_COMMONS_CONNECTION:?Set the private project-role connection file path}"
runtime=${1:?Supply claude, codex, pi or hermes}
shift
case "$runtime" in
  claude|codex|pi|hermes) ;;
  *) echo 'Runtime must be claude, codex, pi or hermes' >&2; exit 2 ;;
esac
exec "${AGENT_COMMONS_BINARY:-agent-commons}" check-in \
  --config "$AGENT_COMMONS_CONNECTION" --runtime "$runtime" "$@"
