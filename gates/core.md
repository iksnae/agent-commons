# Gates: core

Scope: Durable isolated sessions, deliveries, context, and reviewed tasks.

- [x] G1: Domain, restart, isolation and concurrency tests pass.
  CHECK: go test -race ./internal/core
  EXPECT: ok
  EVIDENCE: ok  	agentcommons/internal/core	(cached)
- [x] G2: Four implementation passes complete and final pass finds no improvement.
  EVIDENCE: Implemented full core; domain pass corrected rollback maps and read-side writes; defect pass fixed interrupted result task corruption, stale review revision, managed acknowledgement suppression and accepted-task retry; polish avoids durable writes during idle polling. Final inspection found no further improvement. Regression tests and go vet ./internal/core pass.

Limits: The session methods cover registration, enrolment, the attachment lifecycle, capabilities, policy and listing; none of them removes a session. An identity enrolled by mistake therefore cannot be withdrawn through any supported path and stays in the registry and in every listing. Removing one would have to settle what becomes of its inbox, its assigned tasks and its review verdicts, so this is a design decision rather than a missing method.
