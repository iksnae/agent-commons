# Independent review record

Two independent review passes were performed after implementation. The runtime
builder reviewed core/transport, and the transport builder reviewed runtime;
neither approved its own implementation. The root independently reran the full
test suite, race detector, vet and build after fixes.

## Findings resolved

- Reviewer verdicts implicitly targeted the latest result, allowing stale evidence
  to approve unseen changes. `expectedRevision` now enforces atomic comparison.
- Reading an inbox changed queued work to an unclaimable state. Acknowledgement
  is now separate from execution status.
- Restarting while a lead read a result could demote a submitted/accepted task.
  Recovery only updates an interrupted working task delivery.
- Retrying an obsolete failed delivery could reopen already submitted/accepted
  work. Those retry transitions now reject.
- Cancellation killed a CLI but could leave its MCP children. Owned process-group
  cancellation is tested with an actual forked child.
- Codex's initial tool profile could not inspect files independently. Shell
  access is restored within the read-only sandbox and approval policy.
- Provider failures lost diagnostic evidence. Bounded redacted stderr and
  structured stdout error diagnostics preserve provider refusal codes.
- Project provenance was present only in prompts. Private run receipts now
  record source hashes, selected role, prompt digest, identities and outcome.
- Definition collection lacked a cumulative bound. Inventory now checks the
  total before reading additional content.

Regression checks live in the corresponding core, runtime and transport test
files. Successful RPC logging exposes only authenticated actor and method,
never credential, message body or result contents.

## Independent outcome

Approved for the read-only local pilot, subject to the unpassed full live-provider
gate. No claim of production readiness, arbitrary desktop-session wakeup, hostile
same-user process isolation, full A2A compatibility, or write-capable autonomous
delivery follows from these tests.

Normal CLI exit relies on the provider closing its descendants; cancellation
explicitly terminates its owned process group. The observed live-test processes
exited, with no Agent Commons/MCP child remaining in the process inspection.

Final driver checks: `go test -race -count=1 ./...`, `go vet ./...`, and
`go build -o bin/agent-commons ./cmd/agent-commons` passed across all five packages.

Target inventory was exercised read-only on Khaos Publisher: 21 agent charters,
15 commands and 34 skill entries, with the local reviewer source digest retained.
Claude discovery found Khaos Publisher Lead. Codex discovery returned an explicit
daemon-unavailable diagnostic; no existing Codex app session was adopted.
