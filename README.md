# Agent Commons

Agent Commons is a local coordination service and integration plugin for
project-shaped Claude and Codex teams. It provides scoped identities, inboxes,
shared context and reviewed task results.

![Agent Commons: a shared table connecting independent workspaces](docs/assets/agent-commons-hero.png)

Status: read-only local pilot. The release gates remain open; installing the
plugin never grants repository-write, deployment or operator authority.

## Install

Start in the project where the agent will work. Install the shared harness
instructions with the established Vercel Skills installer:

```sh
DISABLE_TELEMETRY=1 npx skills@1.5.24 add \
  https://github.com/iksnae/agent-commons.git \
  --skill agent-commons --agent claude-code codex pi --copy
```

This is the lowest-friction path. It installs instructions and references; the
plugin reports the next missing prerequisite instead of silently changing
machine state. It does not install the native binary or start the service.

For the actual coordination service, download a trusted native archive for
your operating system and CPU from [Releases](https://github.com/iksnae/agent-commons/releases),
unpack it, and install the bundled product directory. No signed public release
is published yet; the pilot uses a trusted CI artifact or operator-provided
archive.

```sh
/absolute/unpacked/agent-commons bundle install \
  --from /absolute/unpacked \
  --to /absolute/installations/agent-commons
```

The bundle contains the binary, plugin, shared skill, runtime guides, source
notice and licenses. It does not change `PATH`, install global configuration,
start a service or enroll an agent. The complete install, verify and removal
flow is in [INSTALL.md](INSTALL.md).

## Add a native plugin

If you need the native MCP configuration and startup hook, use the copy shipped
in the installed bundle (no clone required):

```sh
DISABLE_TELEMETRY=1 npx skills@1.5.24 add \
  /absolute/installations/agent-commons/plugins/agent-commons \
  --skill agent-commons --agent claude-code codex pi --copy
```

For the native Claude plugin, point Claude at the bundled directory with
`--plugin-dir`. Codex and Pi use their supported local plugin/package installers.
Do not install both a skills copy and a native skill for the same role. The
native plugin starts the scoped MCP command when the harness supports it;
missing binaries, credentials or service access are reported as errors, not
silently repaired. See the [plugin guide](plugins/agent-commons/README.md).

## Let the agent finish setup

From the target project, let the first agent bootstrap its own project-scoped
connection:

```sh
agent-commons init --name lead --role workspace-lead --runtime codex
```

`init` starts the per-user local service when needed, writes `.agent-commons/project.json`
for project defaults, and keeps credentials under the private state directory.
It is safe to run again for the same role. For manual or recovery work, the
lower-level flow remains available:

```sh
export PATH="/absolute/installations/agent-commons:$PATH"
agent-commons enroll --state /private/agent-commons-state \
  --target /absolute/project --name lead --role workspace-lead
agent-commons doctor --config /private/connection.json
agent-commons check-in --config /private/connection.json --runtime claude \
  --native-session ACTUAL_SESSION_ID
```

`doctor` is read-only and reports scoped connection problems without printing
credentials or peer messages. `check-in` returns unread onboarding messages and
current project context without acknowledging anything. Use the [service
guide](SERVICE.md) for a supervised background service. For a foreground pilot,
start `agent-commons serve --state /private/agent-commons-state` in a separate
terminal before running the commands above. Replace `/private/connection.json`
with the connection-file path printed by `enroll`. The full enrollment flow is in
[ONBOARDING.md](integrations/ONBOARDING.md).

## Command center

Open the read-only terminal command center for the enrolled project:

```sh
agent-commons console --config /private/connection.json
```

It shows registered agents and tasks, refreshes every three seconds, and keeps
the service running while you inspect it. Use arrow keys to scroll and `q` to
exit. Add `--once` for one JSON snapshot. The console does not enroll agents,
attach sessions, acknowledge messages, retry work or deploy code. See the
[console guide](docs/console.md).

## Where to go next

- [Runtime support](HARNESS-SUPPORT.md): what Claude, Codex, Pi and Hermes can do.
- [Documentation index](docs/README.md): coordination, notifications and operations.
- [Production readiness](PRODUCTION.md): evidence still required before release.
- [Resume development](START-HERE.md): repository-rooted contributor handoff.

## License

[MPL-2.0](LICENSE). See [LICENSING.md](LICENSING.md) for plain-language terms.
The software comes without warranty or a promise of ongoing support.
