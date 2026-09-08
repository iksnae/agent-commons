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
# The caller may pin the binary; otherwise use the one installed beside this
# plugin, because `bundle install` writes it to the destination root with the
# plugin at <root>/plugins/agent-commons and adds nothing to PATH. A source
# checkout keeps it on PATH, so the bare name remains the last resort and its
# failure stays visible: this is an explicit command, not a launch hook.
binary=${AGENT_COMMONS_BINARY:-}
if [ -z "$binary" ]; then
  installed=${CLAUDE_PLUGIN_ROOT:+$CLAUDE_PLUGIN_ROOT/../../agent-commons}
  if [ -n "$installed" ] && [ -f "$installed" ] && [ -x "$installed" ]; then
    binary=$installed
  else
    binary=agent-commons
  fi
fi
exec "$binary" check-in \
  --config "$AGENT_COMMONS_CONNECTION" --runtime "$runtime" "$@"
