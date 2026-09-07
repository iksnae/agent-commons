# Agent Commons — independent product direction

Operator direction: ship Agent Commons as a standalone binary, usable with any
target repository or multi-project workspace. A private remote in the operator's
iksnae namespace is authorized; public visibility and licensing are undecided.
Claude/Codex integrations should be thin adapters,
not the home of durable domain state.

Product packaging: (1) standalone binary for service/CLI and state lifecycle;
(2) MCP/CLI tools as the stable machine interface; (3) optional thin skills and
runtime configuration for onboarding, launch check-in and usage. Skills explain
the contract but do not own durable state or replace target-owned role definitions.

## Persistent agent identity and launch check-in

The desired lifecycle supports BOTH Claude and Codex at workspace and project
levels. A stable role identity belongs to a target/team/role, not a runtime PID
or conversation UUID. Runtime sessions attach to that identity for a bounded
lease. Workspace and project definitions shape roles; inherited defaults never
overwrite evolved project definitions.

On launch, a configured role should discover the local service, enroll only if
an operator-approved enrollment grant permits it, or reconnect using an existing
private scoped credential; verify target and role; attach its runtime session;
review unread/unhandled inbox items and relevant shared knowledge; report its
availability and arm the appropriate wake adapter. Reading an inbox does not
authorize execution. Existing queued work and knowledge survive runtime exits.

Concurrent launches must not silently steal a role: explicit conflict handling,
lease renewal, expiry, and operator-visible takeover are required. Resuming a
session is distinct from claiming a role. Never adopt by fuzzy name matching.
Scope workspace/project sharing explicitly rather than exposing nested projects
automatically. Tokens stay out of repository configuration and prompts.

Operator-driven idempotent enrollment, private connection files, check-in,
two-minute attachment leases and held renewal are implemented. Stable identity
is project + agent name + role; runtime is an attachment attribute. Enrollment
delivers one-time welcome/getting-started messages. Automatic launch-hook wiring
and delegated enrollment grants remain pending; agents must not self-provision
with operator credentials.

## Product readiness beyond the working pilot

- Versioned persisted schemas, explicit migrations and tested rollback paths.
- Install/uninstall and supervised background service lifecycle per platform.
- Health diagnostics covering service, credentials, bindings, pending/uncertain
  wake attempts, queue reachability and actual receipt separately.
- Signed/reproducible distribution, release versioning and compatibility matrix.
- Durable bounded notification journals, retry reconciliation and queue backpressure.
- Project/workspace launch adapters with tested conflict and offline behavior.
- Explicit publication flow for cross-project knowledge; private boards default.

These are release requirements, not claims that the pilot is production-ready.
