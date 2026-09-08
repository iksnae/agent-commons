#!/bin/sh
# SPDX-License-Identifier: MPL-2.0
set -eu
# The plugin tarball is unpacked wherever its user wants it, so unlike an
# archive that `bundle install` rewrites, it can never know the binary's path at
# packaging time. Resolve it here, at server start, with the same three tiers
# check-in.sh and claude-session-start.sh use: a pinned binary, the one
# installed beside this plugin (`bundle install` writes it to the destination
# root with the plugin at <root>/plugins/agent-commons and adds nothing to
# PATH), and finally the bare name, which is where a source checkout keeps it.
binary=${AGENT_COMMONS_BINARY:-}
if [ -z "$binary" ]; then
  installed=${CLAUDE_PLUGIN_ROOT:+$CLAUDE_PLUGIN_ROOT/../../agent-commons}
  if [ -n "$installed" ] && [ -f "$installed" ] && [ -x "$installed" ]; then
    binary=$installed
  else
    binary=agent-commons
  fi
fi
# Starting an MCP server is an explicit request from the client, not a launch
# hook, so a last-resort failure stays visible rather than exiting quietly.
exec "$binary" connect-mcp "$@"
