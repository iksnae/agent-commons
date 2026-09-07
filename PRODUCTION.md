# Production readiness

Goal: ship Agent Commons for project-shaped Claude and Codex teams, with reliable
coordination across sessions. A local read-only beta is a step toward that goal,
not a substitute for it. Public visibility, deployment and target-agent authority
remain explicit operator decisions.

The preceding native-lifecycle work made verified progress: both hosted operating
systems passed real supervisor tests. The current installer work addresses the
next missing behavior. The production goal remains active.

## Acceptance ledger

| Requirement | Current evidence | What still needs proof |
| --- | --- | --- |
| Stable project + agent name/role identity | Enrollment, conflict and lease tests | Real Claude/Codex project and workspace launch integration without credential sharing |
| Durable communication and return paths | Scoped RPC tests; existing-conversation wake evidence | Full managed cross-runtime acceptance; supervised role watchers and offline recovery |
| Shared context and learnings | Versioned context and scoped board tests | Explicit cross-project publication flow and launch-time review |
| Safe local installation and lifecycle | Native macOS/Linux full service CLI lifecycle CI; bundle boundary tests; recoverable removal and Linux persistent-link checks | Clean-machine upgrades, login/reboot behavior |
| Recovery without lost work or duplicate effects | Durable state, task revision and crash-restart tests | Migration/rollback rehearsal, backup/restore tooling, bounded journals and operator reconciliation |
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
