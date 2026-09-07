# Production readiness

Goal: ship Agent Commons for project-shaped Claude and Codex teams, with reliable
coordination across sessions. A local read-only beta is a step toward that goal,
not a substitute for it. Public visibility, deployment and target-agent authority
remain explicit operator decisions.

Native service lifecycle tests pass on macOS and Linux. Wake maintenance and
operator resolution now have isolated recovery tests; native recovery and the
other gates below remain open. The production goal remains active.

## Acceptance ledger

| Requirement | Current evidence | What still needs proof |
| --- | --- | --- |
| Stable project + agent name/role identity | Enrollment, conflict and lease tests; native Claude init-only hook attachment | Codex launch integration, model inbox recovery, and launcher credential isolation |
| Durable communication and return paths | Scoped RPC tests; existing-conversation wake evidence | Full managed cross-runtime acceptance; supervised role watchers and offline recovery |
| Shared context and learnings | Versioned context and scoped board tests | Explicit cross-project publication flow and launch-time review |
| Safe local installation and lifecycle | Native macOS/Linux full service CLI lifecycle CI; bundle boundary tests; recoverable removal and Linux persistent-link checks | Clean-machine upgrades, login/reboot behavior |
| Recovery without lost work or duplicate effects | Durable state, task revision and crash-restart tests; backed-up wake maintenance and operator-decision tests | Migration/rollback rehearsal, full service backup/restore tooling, bounded journals and native reconciliation verification |
| Project-shaped development teams | Definition inventory and read-only task/review ledger | Approved write-capable dispatch, project isolation, full planning/review/red-team delivery flow |
| Observable operation | Scoped doctor checks; bounded RPC logs | Provider/runtime readiness, supervisor/queue health, actionable failure diagnostics |
| Distributable product | MPL, source-bearing archives, rebuild and CI checks | Signed release artifacts, version/compatibility policy, clean-machine integration installs |

Each open item needs tests at the matching boundary and recorded evidence. A mock
runner cannot prove provider behavior; a parsed service file cannot prove reboot
recovery; a local receipt is not a publisher signature. Independent review remains
required before an implementation increment lands.

See [the beta checklist](BETA.md), [validation](VALIDATION.md), and the original
[acceptance gates](GATES.md). No readiness claim overrides the unpassed live gate
or grants permission to work in a target repository.
