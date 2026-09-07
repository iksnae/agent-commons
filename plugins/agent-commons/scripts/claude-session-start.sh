#!/bin/sh
# SPDX-License-Identifier: MPL-2.0
set -eu
# A plugin may be available without being bound to a role. Never discover tokens.
[ -n "${AGENT_COMMONS_CONNECTION:-}" ] || exit 0
claude_version=$("${AGENT_COMMONS_CLAUDE_BINARY:-claude}" --version)
exec "${AGENT_COMMONS_BINARY:-agent-commons}" launch-context \
  --config "$AGENT_COMMONS_CONNECTION" \
  --agent-type "${AGENT_COMMONS_CLAUDE_AGENT:-}" \
  --claude-version "$claude_version"
