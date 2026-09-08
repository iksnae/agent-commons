# Agent Commons documentation

Start with [installation](../INSTALL.md) and [role enrollment](../integrations/ONBOARDING.md).
For a fresh development session, use [the resume guide](../START-HERE.md).

## Connect a harness

- [Claude](../plugins/agent-commons/guides/claude.md): launch hook, attachment lifetime, fork rejection.
- [Codex](../plugins/agent-commons/guides/codex.md): scoped MCP connection; automatic launch wiring remains open.
- [Pi](../plugins/agent-commons/guides/pi.md): native package installation and exact-session attachment.
- [Hermes](../plugins/agent-commons/guides/hermes.md): experimental visitor metadata; deferred behind Claude and Codex.

## Use and operate

- [Plugin integration](../plugins/agent-commons/README.md)
- [Discovery without adoption](discovery.md)
- [Runtime capabilities and team membership](../HARNESS-SUPPORT.md)
- [Managed process environment](managed-environment.md)
- [Runtime health and queue status](runtime-health.md)
- [Coordination, context and result review](coordination.md)
- [Team-scoped work and revocation](team-work.md)
- [Inbox notifications](notifications.md)
- [Terminal console](console.md)
- [Service installation and control](../SERVICE.md)
- [Wake recovery and maintenance](../WAKE-MAINTENANCE.md)

## Build and decide

- [Builds and archives](builds.md)
- [Version and compatibility policy](version-compatibility.md)
- [Production readiness](../PRODUCTION.md): current delivery priorities and gates
- [Validation evidence](../VALIDATION.md): historical test results, not current permission
- [Implementation contract](../PLAN.md) and [design lineage](../RESEARCH.md)
- [Licensing guide](../LICENSING.md)

Each guide owns its topic. Keep command catalogs in the binary's `help` and
`methods` output rather than copying them into multiple documents.
