# Production readiness

Goal: ship Agent Commons for project-shaped Claude and Codex teams, with reliable
coordination across sessions. A local read-only beta is a step toward that goal,
not a substitute for it. Public visibility, deployment and target-agent authority
remain explicit operator decisions.

Native service lifecycle tests pass on macOS and Linux. Wake maintenance and
operator resolution now have isolated recovery tests; native recovery and the
other gates below remain open. The production goal remains active.

The current completion target covers the remaining production work up to signing,
with Hermes excluded. Signing itself remains a later operator-controlled step;
the other core workflow and operational gates are not waived.

## Delivery priority

The operator has confirmed that Claude and Codex are the core working-team
harnesses. Production effort goes first to their complete team workflow:

1. Launch and resume the intended project role with an explicit credential
   boundary, recover its inbox, and keep its return path available.
2. Delegate work and durably return results across Claude and Codex, including
   offline recipients, cancellation and restart without blind replay.
3. Connect team membership to task/context scope and independently reviewed work,
   then support explicitly authorized writes in isolated project worktrees.

The matching recovery, observability, installation and release gates below still
apply. This priority does not turn a read-only adapter or a passing fixture into
production acceptance.

Hermes is currently a visitor integration, with a later role as a system-level
orchestrator and organizer across workspaces and projects. That role will need
explicit cross-workspace identity and access boundaries; it does not inherit
project credentials or execution authority. Its installer, MCP and launch acceptance work is
deferred behind the core team workflow and does not gate the initial core-team
release. Pi remains foundational support, also behind Claude/Codex delivery.
Neither runtime is being removed from the product; their incomplete capabilities
must remain visible rather than advertised as equivalent to the working teams.

## Core-state recovery guardrails

Optional team-scoped tasks, messages and context now enforce joined membership
through assignment, reads, inbox access, review and result routing. Schema-3
migration preserves a private pre-upgrade snapshot. Service tests cover revocation,
restart and malformed scope records. The supervisor now cancels owned runs when
it observes withdrawn access; subprocess tests verify process-group termination
and withheld results after rejoin. Native provider/tool cancellation acceptance
and transcript/worktree isolation remain open. See
[team-scoped work](docs/team-work.md) for the exact boundary.

Managed execution now filters its environment to launch essentials, configured
network settings and the selected harness's authentication variables. Child-process
fixtures verify that unrelated secrets and the other harness's credentials are
absent. Unsupported provider-routing selectors stop execution before preparation;
they are not silently removed. This does not isolate native homes or credentials
between roles. See [the exact environment contract](docs/managed-environment.md).

Managed results cannot replace an already bound native session ID. A mismatched
runner result is recorded as failed, its output is withheld from successful
return delivery and task submission, and the original binding survives restart.
Claude/Codex adapter fixtures and durable service regression tests exercise this
boundary. They do not prove native resume or provide an operator rebind workflow.

Preparation/checkpoint components now have isolated tests: a prepared thread can
be bound to a running delivery before execution, checkpoint failure stops the
runner, and the binding survives interruption without replay. Ready-journal reuse
checks native scope; partial journals stop without creating a replacement.

These components are not wired into `serve`. Independent review found that the
standalone app-server starter loads native user/project configuration, unlike the
managed execution policy. Codex 0.153.4 rejects app-server `--ignore-user-config`
and `--ignore-rules`; the native clean-profile preparation test does not prove
configuration isolation. Automatic first-run preparation remains an open gate.
The next integration must prove that unwanted MCP/plugin components cannot start
and preserve the same native home through execution without copying credentials.

Role credential isolation, Claude first-run preparation, durable journal
reconciliation and native model-turn recovery remain open. Do not delete
incomplete journals to force a fresh start.

Codex preparation and ready-binding reuse now share the resume lock until their
owned native process closes. Tests cover competing acquisition before and after
shutdown, including partial preparation. This lock only coordinates cooperating
binding users; whole-service backup still needs a quiescence protocol that covers
new journal creation, watchers and core state.

Existing state is decoded without fresh-service defaults. Empty, null and
missing-map files are rejected without replacing them. A missing operator
credential in an existing file is also an error; startup does not invent a
replacement. A genuinely absent state file still creates a new service.

`state.json` must be a private regular file and no larger than 64 MiB. Startup
rejects special files without waiting for a writer. The same encoded-size limit
applies before saving mutations, so a rejected oversized change preserves the
previous disk state and rolls back the in-memory change. Existing snapshots above
the limit are refused, not truncated or migrated automatically.

This cap is not retention management or complete corruption detection. Preserve
the state and its backups if the limit is reached; do not strip messages or token
records by hand. Archival, full backup/restore tooling and validation of every
persisted record remain work to do. No live pilot state was opened or changed to
exercise these checks.

## Acceptance ledger

| Requirement | Current evidence | What still needs proof |
| --- | --- | --- |
| Stable project + agent name/role identity | Enrollment, conflict and lease tests; native Claude init-only hook attachment | Codex launch integration, model inbox recovery, and launcher credential isolation |
| Core Claude/Codex working-team harnesses | Shared capability catalog, native Claude startup attachment and Codex preparation/resume checks; read-only managed adapters | End-to-end native team participation, launcher credential isolation, reliable continuation and restart acceptance |
| Secondary Pi/Hermes participation | Explicit check-in and project-source inventory tests; Pi local package install, startup, seeded-root resume, fork rejection and removal; Hermes 0.21.0 portable loader reads a copied skill/MCP bundle | Deferred behind core teams: native dispatch, credential isolation, Hermes installer lifecycle, MCP execution, launch, wake and fresh-session recovery acceptance |
| Durable communication and return paths | Scoped RPC tests; existing-conversation wake evidence | Full managed cross-runtime acceptance; supervised role watchers and offline recovery |
| Shared context and learnings | Versioned context and scoped board tests; newest-first check-in for all four harnesses with older-service compatibility | Explicit cross-project publication flow, durable review tracking and native launch-time review |
| Safe local installation and lifecycle | Native macOS/Linux full service CLI lifecycle CI; bundle boundary tests; recoverable removal and Linux persistent-link checks | Clean-machine upgrades, login/reboot behavior |
| Recovery without lost work or duplicate effects | Durable state, task revision and crash-restart tests; backed-up wake maintenance and operator-decision tests | Migration/rollback rehearsal, full service backup/restore tooling, bounded journals and native reconciliation verification |
| Project-shaped development teams | Definition inventory, read-only task/review ledger, explicit membership and team-scoped task/context/result routing with revocation and restart tests | Native launch-time participation, approved write-capable dispatch, native-session/project isolation, full planning/review/red-team delivery flow |
| Observable operation | Scoped doctor checks; bounded RPC logs; process-local supervisor heartbeat and bounded, content-free queue/cancellation counts | Provider readiness, complete actionable failure diagnostics and native operational acceptance |
| Distributable product | MPL, source-bearing archives, rebuild and CI checks; pinned Vercel Skills install/removal preserves shared skill and support files at all four harness paths; version and compatibility policy documented | Signed release artifacts, clean-machine native integration installs |

Each open item needs tests at the matching boundary and recorded evidence. A mock
runner cannot prove provider behavior; a parsed service file cannot prove reboot
recovery; a local receipt is not a publisher signature. Independent review remains
required before an implementation increment lands.

See [the beta checklist](BETA.md), [validation](VALIDATION.md), and the original
[acceptance gates](GATES.md). No readiness claim overrides the unpassed live gate
or grants permission to work in a target repository.
