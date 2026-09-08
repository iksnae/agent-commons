# Gates: core

Scope: Durable isolated sessions, deliveries, context, and reviewed tasks.

- [x] G1: Domain, restart, isolation and concurrency tests pass.
  CHECK: go test -race ./internal/core
  EXPECT: ok
  EVIDENCE: ok  	agentcommons/internal/core	(cached)
- [x] G2: Four implementation passes complete and final pass finds no improvement.
  EVIDENCE: Implemented full core; domain pass corrected rollback maps and read-side writes; defect pass fixed interrupted result task corruption, stale review revision, managed acknowledgement suppression and accepted-task retry; polish avoids durable writes during idle polling. Final inspection found no further improvement. Regression tests and go vet ./internal/core pass.

Limits: An identity enrolled by mistake is withdrawn with operator-only `sessions.retire {id,evidence}`, and returned with `sessions.reinstate {id,evidence}`. Retirement is a tombstone, not a delete: the `Session` record stays with its Name, Role and Target, and only the credential is destroyed. Nothing is erased, and there is no delete, purge or force flag. The inbox stays readable through `inbox.list {sessionId}` with its acknowledgement flags as they were, board posts keep their author, review verdicts stay verbatim in `Task.Reviews`, and team memberships become `revoked` rather than disappearing. Undelivered work addressed to the identity becomes `interrupted`, never `completed`. Retired identities are excluded from `sessions.list` by default and returned by operator-only `{includeRetired:true}`.

The new limit is that withdrawal can be blocked. Retirement is refused while the identity is busy, while it holds a live attachment lease (the lease is 120 seconds; wait or `sessions.detach`), and while it participates as lead, author, reviewer or red team in a task that has reached neither `accepted` nor `abandoned` — that refusal names the blocking task IDs. There is no override: retiring a designated reviewer must never dissolve a review obligation, so a stuck task has to be resolved or abandoned first. Reinstatement issues a new credential; the destroyed one never comes back, and the private connection and credential files on disk are reported as inert rather than deleted.
