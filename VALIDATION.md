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

Opt-in native launchd and systemd tests have passed on hosted macOS and Linux:
isolated service startup, SIGKILL recovery to a different PID, durable registry
verification, stop and transient
unregistration, plus clean stop/start and explicit registration-absence checks.
No provider sessions were launched. Login/reboot persistence remains unverified.
The dedicated manual CI workflow keeps these
host-dependent tests separate from ordinary builds.
Evidence: [native lifecycle run 34147092700](https://github.com/iksnae/agent-commons/actions/runs/34147092700).

Local bundle installation now has boundary tests for exclusive destinations,
symlink rejection, receipt verification and recoverable removal. The archive check
installs, verifies, runs help and removes the host-native bundle. This installs
files only; it does not register a persistent service, migrate state or enroll roles.

The separate `service` CLI now installs configuration and controls explicit user
jobs. Filesystem tests cover existing-file collisions, incomplete receipts,
changed-file refusal and recoverable removal. Fake supervisor tests cover command
failure and state preservation. The full native CLI path passed on both macOS
(10.73 seconds) and Linux (11.18 seconds) in
[run 34150065040](https://github.com/iksnae/agent-commons/actions/runs/34150065040),
with neither test skipped. It verified install, enable, start, status, SIGKILL
recovery, clean restart, stop, recoverable removal and retained agent state.
Linux persistent startup links were present after enable and absent after removal.
Login/reboot recovery remains unverified; see [service setup](SERVICE.md).

Claude launch integration has an opt-in native test using `--init-only`, a private
Claude configuration, and a disposable enrolled project. With Claude Code 2.1.236
on macOS, it passed in 0.36 seconds: the plugin's SessionStart hook attached the
role while welcome messages remained unread. No model conversation ran. Parser
tests cover version floors, fork/subagent rejection and bounded events; integration
tests cover identity reuse and project mismatch. Codex startup, automatic renewal,
and inherited credential isolation remain open. This test does not demonstrate
that a model read or obeyed the hook's inbox guidance.

`just codex-hook-test` passed against Codex CLI 0.153.4 on macOS. A disposable
stdio app-server discovered an isolated launch hook as enabled but untrusted,
with its definition hash. This proves native inventory and trust metadata only;
no hook was trusted or executed and no model turn ran. Packaged hook loading,
fork-safe identity binding and launch-time inbox recovery remain open. See
[Codex launch acceptance](gates/codex-launch.md).

The native Codex test now also installs the product plugin from a disposable local
marketplace. Codex 0.153.4 lists the MCP server and namespaced skill, makes the
installed skill available to the target, and copies every bundled file unchanged.
Uninstall clears installed status and removes the cache. The Claude-only hook is
absent from Codex's plugin metadata. These checks do not exercise an MCP connection,
model turn or Codex launch hook. No plugin was installed in the operator's config.
