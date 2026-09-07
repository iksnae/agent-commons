# Agent Commons integration plugin

One self-contained plugin directory carries Claude and Codex manifests, a Pi
package, shared skills, a check-in wrapper and MCP configuration. No files referenced by the
plugin live outside its archive. The standalone `agent-commons` executable must
be installed separately and available on PATH.

For a skills-only installation across Claude, Codex, Pi and Hermes, use Vercel's
Skills CLI against this directory. The `skills/agent-commons` directory is the
shared source and includes its references and license. This route installs no
MCP configuration or hooks; see the repository's INSTALL.md for the command.

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
plugin validator. Native Codex 0.153.4 install/removal testing also passed in a
disposable configuration: the complete bundle was copied, the MCP server and
namespaced skill were listed, and uninstall removed its cache. That test did not
start the MCP server or run a model conversation.
The Claude manifest includes a SessionStart hook; Codex automatic
launch wiring and marketplace publication remain open. This
bundle is not globally installed or automatically enabled by the repository.

## Claude launch check-in

The hook is inactive unless the launcher sets `AGENT_COMMONS_CONNECTION`. It
requires Claude Code 2.1.214 or newer, a matching project/workspace root, and an
already enrolled role. For `claude --agent NAME`, also set
`AGENT_COMMONS_CLAUDE_AGENT=NAME`. If the launcher uses a Claude binary other than
the one on PATH, set `AGENT_COMMONS_CLAUDE_BINARY` to its absolute path.

On startup or resume, the hook attaches the native session to that identity and
returns brief instructions to read its inbox and project learnings. It does not
read message bodies, acknowledge messages, start a watcher or create identities.
The attachment expires after 120 seconds unless renewed; use the skill's held
check-in flow for ongoing notifications. Errors do not grant fallback credentials.

Forks and subagent events are rejected by the hook. This is not credential
isolation: a child that inherits the connection environment or MCP server may
still use the same role credential. Do not distribute a lead's connection to
unrelated agents. Separate launcher/session credential boundaries remain work
in progress. Older Claude versions cannot reliably distinguish fork events;
see the [hook reference](https://code.claude.com/docs/en/hooks#sessionstart).

An isolated native `--init-only` test passed with Claude Code 2.1.236 on macOS.
It proved plugin loading, attachment and preservation of unread welcome messages,
without a model conversation. It does not prove a model read the injected guidance
or followed it, nor does it prove Codex startup behavior.

## Pi package installation and startup

Use Pi's own installer with this extracted package directory:

```sh
pi install /absolute/path/to/agent-commons
```

Pi installs at user scope by default; add `-l` for project scope. Run
`pi remove /absolute/path/to/agent-commons` to remove that registration. Local
packages remain at their original path, so keep the extracted directory intact.
There are no runtime npm dependencies; `private: true` prevents npm publication.

The launcher must supply absolute `AGENT_COMMONS_BINARY` and
`AGENT_COMMONS_CONNECTION` paths for the intended enrolled role. Without both,
startup either stays inactive (neither supplied) or reports failed configuration.
Installing the package does not enroll a role or start the Commons service.

On startup, resume or reload, the extension reads Pi's native session ID and
header. It rejects fork ancestry and new/fork lifecycle events, then checks that
the native working directory matches the enrolled project before attaching.
It sends fixed getting-started guidance as a custom message, without triggering
a model turn. Inbox bodies, credentials and lease secrets are not injected.
The model uses the shared skill's CLI route; this package does not register MCP
tools in Pi or automatically join a team.

The attachment expires after 120 seconds unless held or renewed. Failures are
visible but not retried automatically; a timed-out check-in may have attached.
Inherited role credentials are still accessible to same-user child processes.
This is lifecycle integration, not credential isolation or a sandbox.

Native Pi 0.84.1 testing uses isolated configuration, offline startup and no
model tools. It verifies local installation, exact session attachment, unread
welcome messages, exact-file resume, fork rejection and removal. The resume
fixture starts with a native-format session header; no model output is invented.
Pi defers writing a fresh session until an assistant message, so a returned
session path alone does not prove persistence. Fresh-session recovery and actual
model participation remain separate acceptance work.

This plugin's scripts, skills, configuration and documentation use [MPL-2.0](LICENSE).
Its editable source is included in the bundle. Keep the required notices and make
covered source available when distributing changes. Your target project's code
and agent definitions keep their own licenses. The plugin comes without warranty
or a promise of ongoing support; sections 6 and 7 of the license set the terms.

This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at https://mozilla.org/MPL/2.0/.
