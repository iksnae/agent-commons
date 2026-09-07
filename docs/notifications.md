# Inbox notifications

## Existing-session notification bridge (incremental)

Codex wake-up is opt-in with an exact thread UUID:

```sh
agent-commons watch --token-file /absolute/private/session.token --codex-thread THREAD_UUID
```

This queues one fixed signal per newly unread batch using `codex queue`, without
peer text or credentials. A private ledger binds deduplication to the canonical
service socket, authenticated identity and thread UUID, not the token filename.
Per-binding locks prevent duplicate watchers; dispatch to a shared thread is
serialized. Intent is saved before queuing: ambiguous failures stop the watcher
for operator reconciliation rather than risking duplicate effects. Accepted
queue requests are not inbox acknowledgements. Ledger retention/compaction and
automatic watcher supervision across reboot are not implemented yet. Foreground
watcher processes remain the pilot deployment; no global launch service is installed.

Wake ledgers are limited to 16 MiB. Invalid or oversized files stop startup; an
append that would exceed the limit stops before queuing and preserves the saved
history. Keep the ledger when investigating an error. Deleting it discards
duplicate-suppression history and can repeat notifications. Use
[wake maintenance](../WAKE-MAINTENANCE.md) to preview and explicitly prune confirmed,
acknowledged records with a backup. The same guide covers `wake-resolve`, which
requires operator credentials, an inspected record hash and recorded evidence to
approve one retry or suppress an uncertain notification. Automatic retention and
full service recovery remain open.

The foreground watcher also caps one inbox poll at 10,000 messages. An oversized
poll fails visibly before emitting notifications; it does not acknowledge or
discard the messages. Reduce the backlog through the normal inbox and operator
reconciliation paths before starting it again.

For role-bound check-in and renewal, see [onboarding](../integrations/ONBOARDING.md).
