# Validation status

Automated tests, race checks, vet and build pass for the service, CLI, transports,
runtime adapters, scoped knowledge board, attachment leases and onboarding.
The onboarding refactor received independent review and focused lifecycle tests.

Live evidence established existing Claude leads reading and replying through
scoped inboxes. A fixed `codex queue` signal started a subsequent turn in an
existing conversation. Integrated inbox arrival → watcher → queue → turn → read
and acknowledgement was also exercised. Operational transcripts, native session
IDs and local credential paths are deliberately excluded from version control.

These results do not prove complete managed-runtime bidirectional acceptance:
one earlier managed round trip completed, but a later resumed Claude reply was
refused by its provider. That attempt was not retried with a different model or
otherwise worked around. The associated G5 gate remains open.

Enrollment and check-in are tested; unattended launch hooks have not been
installed. Wake watchers are foreground pilot processes, not reboot-supervised
services. Retention, reconciliation of uncertain queue attempts, complete state
migrations/rollback and signed release distribution remain work for
product release. MPL-2.0 licensing and source-bearing binary archives are now in
place; archive checks rebuild all four binaries from bundled source. See
[the beta checklist](BETA.md) for remaining operational gates.
Role attachment leases prevent accidental concurrent takeover;
they are not process isolation for callers sharing the same role credential.

Beta preparation adds a read-only `doctor` command and `service-plan` renderers
for launchd and systemd user services. Tests cover scoped identity checks, no
credential/peer-text output, no attachment or acknowledgement, rejected FIFO
config files, service-file escaping and native parser validation where available.
An isolated subprocess crash/restart test verifies registry persistence and
default socket recovery. These tests do not prove installed supervisor lifecycle,
reboot recovery or a migration of the running pilot.
