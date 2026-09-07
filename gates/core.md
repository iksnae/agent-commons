# Gates: core

Scope: Durable isolated sessions, deliveries, context, and reviewed tasks.

- [x] G1: Domain, restart, isolation and concurrency tests pass.
  CHECK: go test -race ./internal/core
  EXPECT: ok
  EVIDENCE: ok  	agentcommons/internal/core	(cached)
- [x] G2: Four implementation passes complete and final pass finds no improvement.
  EVIDENCE: Implemented full core; domain pass corrected rollback maps and read-side writes; defect pass fixed interrupted result task corruption, stale review revision, managed acknowledgement suppression and accepted-task retry; polish avoids durable writes during idle polling. Final inspection found no further improvement. Regression tests and go vet ./internal/core pass.
