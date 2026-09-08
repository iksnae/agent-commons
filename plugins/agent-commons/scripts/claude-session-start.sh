#!/bin/sh
# SPDX-License-Identifier: MPL-2.0
set -eu
# A SessionStart hook must never break a launch, so the only hard requirement is
# a binary to run. The launcher may pin one; otherwise use the binary installed
# beside this plugin, because `bundle install` writes it to the destination root
# with the plugin at <root>/plugins/agent-commons and adds nothing to PATH; and
# otherwise fall back to PATH, which is where a source checkout keeps it.
binary=${AGENT_COMMONS_BINARY:-}
if [ -z "$binary" ]; then
  installed=${CLAUDE_PLUGIN_ROOT:+$CLAUDE_PLUGIN_ROOT/../../agent-commons}
  if [ -n "$installed" ] && [ -f "$installed" ] && [ -x "$installed" ]; then
    binary=$installed
  elif command -v agent-commons >/dev/null 2>&1; then
    binary=agent-commons
  elif [ -n "${AGENT_COMMONS_CONNECTION:-}" ]; then
    # The operator named a connection, so they meant this launch to check in.
    # Report the missing binary rather than swallowing their intent. Never
    # exit 2: Claude Code treats 2 as blocking and this must not stop a launch.
    echo 'agent-commons: AGENT_COMMONS_CONNECTION is set but no agent-commons binary was found (AGENT_COMMONS_BINARY, $CLAUDE_PLUGIN_ROOT/../../agent-commons, PATH)' >&2
    exit 127
  else
    # The plugin can be present without the binary, and most launches are in
    # projects that were never enrolled. Nothing to do; stay quiet.
    exit 0
  fi
fi
# A plugin may be available without being bound to a role. Never discover
# tokens: pass --config only when the launcher supplied a connection, and
# otherwise let the binary resolve the role itself by searching upward from the
# launch directory for .agent-commons/project.json.
set -- launch-context
[ -z "${AGENT_COMMONS_CONNECTION:-}" ] || set -- "$@" --config "$AGENT_COMMONS_CONNECTION"
claude_version=$("${AGENT_COMMONS_CLAUDE_BINARY:-claude}" --version)
exec "$binary" "$@" \
  --agent-type "${AGENT_COMMONS_CLAUDE_AGENT:-}" \
  --claude-version "$claude_version"
