# Maintaining wake history

The wake ledger suppresses repeated Codex arrival signals for one thread and
enrolled identity. It has a 16 MiB limit. Maintenance can shrink the active ledger
without changing messages, attachment leases or task execution.

Stop the matching watcher before maintenance. The command takes the same binding
lock and refuses to run while that watcher holds it. Even preview may create a
lock file; it does not change the ledger or inbox.

Preview with the enrolled role's private connection file and exact thread UUID:

```sh
agent-commons wake-maintain --config /private/connection.json --codex-thread THREAD_UUID
```

Only `queued` records with an acknowledged receipt in that role's current inbox
are eligible. Unread, missing, attempting and uncertain records remain. All inbox
pages must succeed before anything changes. Acknowledgement means read, not handled
or executed; maintenance does not change either state.

Apply the same plan against a fresh inbox read:

```sh
agent-commons wake-maintain --config /private/connection.json --codex-thread THREAD_UUID \
  --apply --acknowledge-replay-risk
```

The command saves and syncs a private `.wake-backup-*.json` file before replacing
the active ledger. Its JSON report includes eligible and planned remaining counts,
whether application completed, and the backup path. `uncertain:true` means a write
failed and the ledger must be inspected before retrying. Backups are retained even
after success; they contain notification IDs and statuses, not message bodies.
This command bounds the active ledger, not total disk use including backups.

Restoring older service state can make pruned notifications eligible again. Keep
the backup with the corresponding service recovery records. Do not delete or
blindly replace the ledger to clear an error. This tool cannot repair malformed or
already-oversized ledgers, resolve uncertain dispatches, or migrate the running
pilot. Those recovery paths still need operator tooling and verification.
