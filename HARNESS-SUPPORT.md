# Runtime harness support

Agent Commons has foundational integrations for Claude Code, Codex, the
[Pi coding agent](https://github.com/earendil-works/pi), and
[Hermes Agent](https://github.com/NousResearch/hermes-agent).
Run `agent-commons harnesses` for the implemented capability catalog. These flags
describe Agent Commons adapters, not everything an upstream agent can do.

The intended workflow is team participation: an agent joins a coordination team
session, receives the team brief and relevant knowledge, and contributes through
its inbox and task/review responsibilities. Its native runtime session stays
separate from that shared coordination space. Current check-in support is the
foundation; the complete join/rejoin/membership workflow still needs implementation
and acceptance tests. Later cross-team boards and inboxes will provide a separate,
explicit sharing scope for learning and experiments beyond project work.

| Runtime | Explicit role check-in and shared coordination | Managed dispatch adapter | Existing-session arrival signal |
| --- | --- | --- | --- |
| Claude Code | Yes | Read-only adapter; live acceptance gate remains open | Not implemented |
| Codex | Yes | Read-only adapter; live acceptance gate remains open | Fixed queue signal |
| Pi | Yes | Not implemented | Not implemented |
| Hermes | Yes | Not implemented | Not implemented |

An operator enrolls a project + agent name/role once. Its runtime sessions attach
to that identity using the role's private connection file:

```sh
agent-commons check-in --config /private/connection.json --runtime pi --native-session EXACT_PI_SESSION_ID
agent-commons check-in --config /private/connection.json --runtime hermes --native-session EXACT_HERMES_SESSION_ID
```

Supply the actual session ID. Agent Commons does not discover or adopt a Pi or
Hermes session from a name, a recent-session selector, or an inherited Codex ID.
The attachment lease, target restrictions, durable inbox, board and task-review
rules are shared across harnesses. Check-in returns inbox data but does not
acknowledge it. Acknowledge only messages actually read.

`--hold` renews the attachment and emits arrival events. For Pi and Hermes those
events do not automatically start a model turn. Their native notification and
launch integrations remain work to do. Managed registration is rejected while
the corresponding dispatch adapter is unavailable; work is not silently queued
for a nonexistent runner.

## Project-owned definitions

Inventory reads `.pi/skills/`, `.pi/prompts/`, `.hermes/skills/` and shared
`.agents/skills/`, alongside the existing Claude/Codex/AgencyX sources. It keeps
source paths, reference directories and hashes. Pi prompts appear as commands in
the common inventory. Inventory does not install files or grant native trust.
Hermes requires a separate project-skill trust decision; Agent Commons does not
make it. See [Pi configuration](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/README.md)
and [Hermes project skills](https://hermes-agent.nousresearch.com/docs/user-guide/features/skills#project-local-skills).

## Native adapter work still required

Pi exposes JSON/RPC modes and exact-session selection. Hermes exposes CLI session
resume and ACP. These are integration candidates, not implemented dispatch paths
in this release. Hermes's installed one-shot help says it bypasses approvals, so
it must not be substituted for an approval-preserving managed adapter. See the
[Hermes CLI guide](https://hermes-agent.nousresearch.com/docs/user-guide/cli).

Both harnesses still need native result/failure parsing, explicit session resume,
credential isolation, project tool restrictions, cancellation/restart tests, and
launch/wake verification. Existing locally configured Pi extensions do not prove
those integrations are bundled with Agent Commons. No provider was installed,
configured, trusted, or launched into a model turn while adding this foundation.
