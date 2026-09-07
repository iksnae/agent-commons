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
migrations/rollback, signed release distribution and licensing remain work for
product release. Role attachment leases prevent accidental concurrent takeover;
they are not process isolation for callers sharing the same role credential.
