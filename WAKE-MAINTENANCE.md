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

Only `queued` or operator-`suppressed` records with an acknowledged receipt in that role's current inbox
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
after success; they contain notification IDs, statuses and any operator evidence.
Message bodies and credentials are not copied automatically. Keep secrets out of
operator evidence, which is also visible in shell history when supplied as a flag.
This command bounds the active ledger, not total disk use including backups.

Restoring older service state can make pruned notifications eligible again. Keep
the backup with the corresponding service recovery records. Do not delete or
blindly replace the ledger to clear an error. This tool cannot repair malformed or
already-oversized ledgers or migrate the running pilot.

## Resolving an uncertain notification

An `attempting` or `uncertain` record stops automatic retries because the runtime
may already have received the signal. Review the runtime history before choosing
whether to retry or suppress that notification. Stop the matching watcher first;
resolution uses its binding lock too.

Inspect one record without changing it:

```sh
agent-commons wake-resolve --config /private/connection.json \
  --codex-thread THREAD_UUID --message-id MESSAGE_ID
```

The report returns `inspectedStatus` and `recordHash`. Apply a decision against
that exact hash, using an explicit private operator credential for the same service:

```sh
agent-commons wake-resolve --config /private/connection.json \
  --codex-thread THREAD_UUID --message-id MESSAGE_ID \
  --expected-hash HASH_FROM_INSPECTION --decision retry \
  --evidence 'Reviewed runtime history; accepting the duplicate notification risk.' \
  --operator-token-file /private/operator.token --acknowledge-notification-risk
```

`retry` records approval for one later watcher attempt. This command does not send
a signal or start a model. If that later attempt fails, it becomes uncertain again
and needs a fresh decision. Use `--decision suppress` to prevent another signal
for this record, accepting that it may never have arrived. Neither decision
acknowledges the inbox, marks work handled, or authorizes task execution.

A role credential alone cannot apply either decision. Changed records reject old
hashes. Each change saves a private backup first and retains the decision, its
timestamp, evidence and previous record hash. Later dispatch status changes keep
that evidence. Older decisions remain in backups; this is local recovery history,
not a tamper-proof audit log.

The report describes the inspected record. `applied:true` means the replacement
was saved; `uncertain:true` means a write failed and you must inspect the ledger
and retained backup before taking another action. Restoring older state can make
an old hash valid again, so do not treat it as an exactly-once guarantee.

Resolution adds metadata and statuses that older binaries may not understand.
Upgrade the watcher and maintenance binary together. Do not run an older watcher
against a resolved ledger. Keep backups for a coordinated recovery; automatic
downgrade, malformed-ledger repair and full service backup/restore are not provided.
