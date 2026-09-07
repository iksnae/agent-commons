# Runtime health and queue status

`doctor --config /absolute/private-role-connection.json` now includes a runtime
snapshot when the connected service advertises `runtimeStatusAvailable`. Its
`ready` field still means the scoped RPC checks passed, not that a provider is
available or the product is production-ready. Older services omit this snapshot.

Agents can call the read-only `runtime.status` tool for their own identity. They
cannot select another identity, even in the same project. Operators can inspect
all registered identities, optionally filtering by `sessionId` or registered
`target`, using the existing authenticated CLI:

```sh
agent-commons call --socket /absolute/state/service.sock \
  --token-file /absolute/state/operator.token runtime.status '{"limit":25}'
```

The default page holds 25 identities; the maximum is 100. Follow `nextCursor`
with the same filters until it is empty. Roles, credentials and message receipts
are unchanged by these calls. Output excludes message/error text, delivery IDs,
provider credentials and native session IDs. `nativeBound` is only a boolean.

## Supervisor observation

The response distinguishes `not_observed`, `responsive`, `stale` and `stopped`.
The running supervisor reports a heartbeat each dispatch loop. It becomes stale
after three polling intervals plus one second without a heartbeat. `lastSeen`,
`ageMillis` and `pollMillis` explain that observation. No provider request is made.

Observations are process-local and reset on service restart. A second supervisor
cannot reserve the same in-process slot while the first is running. A responsive
loop does not prove model availability, credential validity or progress inside a
long-running provider call. During shutdown, the last heartbeat can become stale
while children are still being joined; `stopped` follows that join.

## Queue counts

`ready` counts pending managed deliveries eligible under current team membership.
`waitingTeam` counts pending deliveries blocked by that membership. `blockedPolicy`
counts tasks whose author's current policy no longer permits workflow execution.
Those need operator repair; they are not ready to run. `manualPending`
counts pending manual deliveries. Other counters reflect durable delivery status:
`running`, `interrupted`, `failed`, `completed` and `acknowledged`.

`canceled` is a subset of `failed`, recorded when the managed supervisor observes
cancellation. It is not inferred from provider error text. Older failures lack
that classification and remain in `failed`. An explicit operator retry clears the
old classification; this report does not trigger retries.

Counts include retained deliveries addressed to the selected identity, including
team-scoped work that identity can no longer read. They reveal no team IDs or
content. Interrupted work needs operator reconciliation because an earlier run
may have acted. Failed work needs authorized inbox inspection; do not blindly
retry it. A team wait requires membership coordination, not broader credentials.

These are bounded snapshots, not a metrics history or a provider readiness probe.
Full provider diagnostics and native operational acceptance remain open.
