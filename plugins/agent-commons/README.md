# Agent Commons integration plugin

One self-contained plugin directory carries Claude and Codex manifests, shared
skills, a check-in wrapper and MCP configuration. No files referenced by the
plugin live outside its archive. The standalone `agent-commons` executable must
be installed separately and available on PATH.

Before enabling the plugin for a role, the operator enrolls project + name + role
and sets `AGENT_COMMONS_CONNECTION` in that role's launcher environment to the
private connection-file path. The MCP server checks that the configured identity
matches its credential. Neither plugin installation nor that environment variable
creates an identity or grants operator permissions.

`scripts/check-in.sh claude --native-session ACTUAL_ID` and
`scripts/check-in.sh codex` perform minimal check-in. Add --hold to renew the
attachment and watch arrivals. The model reads its welcome/getting-started inbox
messages for the next steps. Do not enable one shared role connection for every
subagent in a project.

Manifest validation has been exercised with Claude's validator and the Codex
plugin validator. Marketplace publication, installer testing and automatic
SessionStart hook wiring are not complete. This bundle is not globally installed
or automatically enabled by the repository.

This plugin's scripts, skills, configuration and documentation use [MPL-2.0](LICENSE).
Its editable source is included in the bundle. Keep the required notices and make
covered source available when distributing changes. Your target project's code
and agent definitions keep their own licenses. The plugin comes without warranty
or a promise of ongoing support; sections 6 and 7 of the license set the terms.

This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at https://mozilla.org/MPL/2.0/.
