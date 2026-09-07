# Local beta checklist

We have a working read-only pilot. This checklist tracks the work needed before
we call it a supervised local beta. Passing unit tests alone does not close an
operational gate.

## Verified foundations

- Durable scoped identities, inboxes, shared knowledge and result review gates.
- Enrollment and check-in tests, including lease conflicts and renewal failure.
- Existing-conversation inbox arrival, Codex wake signal, read and acknowledgement.
- MPL-2.0 licensing, native archives with source and third-party notices, and
  byte-for-byte rebuild checks from bundled source.
- An isolated subprocess test kills the service, restarts it, and verifies that
  the registry survives and the stale default socket is recovered. Active sockets,
  custom paths, regular files and symlinks are not removed by recovery.
- A native macOS launchd test starts an isolated service, kills it, observes a new
  PID with the same registry, and stops/unregisters its temporary job. This is
  session-local supervision evidence, not a login/reboot or production migration test.

## Still required for the beta

- [ ] Service install/start/stop/uninstall tested on macOS and Linux without
  touching unrelated jobs or deleting state. `service-plan` now renders user-job
  files for review; it does not install them. Native parser tests are not lifecycle tests.
  macOS transient lifecycle is exercised; Linux lifecycle and a user-facing
  installer remain open.
- [ ] Role check-in and wake processes supervised, including offline startup and
  uncertain queue results. A service restart must not silently replay an uncertain wake.
- [ ] Explicit project/workspace launch integration tested with both Claude and
  Codex. Credentials must not leak into unrelated subagents.
- [ ] Existing private state backed up, migration/rollback rehearsed, then an
  explicitly scheduled switch from the old foreground service.
- [ ] Clean-machine install and recovery tests; diagnostics for provider access,
  queue state and supervisor health. `doctor` currently checks scoped RPC access only.
- [ ] Full bidirectional managed-runtime acceptance. The prior provider refusal
  remains recorded, not bypassed or counted as a pass.
- [ ] Notification retention, backpressure and operator reconciliation for
  uncertain deliveries.

Signed public releases and write-capable autonomous teams are later release work.
The local beta will remain read-only. Repository visibility does not change when
a checklist item passes.

## Read-only diagnostics

`agent-commons doctor --config /private/connection.json` checks private connection
files, credential/identity binding, inbox access and board access. It emits a
bounded JSON report and exits nonzero on failure. It does not attach a session,
acknowledge messages, start a model or print peer messages and credentials.
`ready: true` means those connection checks passed, not that the product is ready.

## Review a service plan

```sh
agent-commons service-plan --platform darwin \
  --binary /absolute/installed/agent-commons \
  --state /absolute/private/agent-commons \
  --path /absolute/provider/bin:/usr/bin:/bin
```

Use `--platform linux` for a systemd user unit. Output is JSON with the filename,
service contents and a scope warning. Neither command writes service files.
Choose a stable binary path outside a checkout, and an explicit PATH containing
the authenticated provider CLIs. The plan uses a private umask and restarts failed
service processes. It does not supervise watchers or make model credentials available.
On startup, the CLI recovers a stale default `STATE/service.sock` only after
acquiring the exclusive state lock, checking current-user ownership and probing
for a refused connection. Custom socket paths require operator cleanup. This is
not protection against a hostile process running as the same OS user.

macOS user agents run in the logged-in user's session. A Linux user unit also
depends on user-manager lifecycle; no lingering or boot-time guarantees are set up.
See [Apple's launchd guide](https://developer.apple.com/library/archive/documentation/MacOSX/Conceptual/BPSystemStartup/Chapters/CreatingLaunchdJobs.html)
and [systemd's service reference](https://www.freedesktop.org/software/systemd/man/latest/systemd.service.html).

## Exercise a native supervisor

Build the binary, then set `AGENT_COMMONS_NATIVE_SUPERVISOR=1` and
`AGENT_COMMONS_TEST_BINARY` to its absolute path when running
`go test -race ./integration -run '^TestNativeSupervisorLifecycle$' -count=1 -v -timeout 8m`.
This opt-in test really registers and stops a temporary user-service job. It copies
the binary into a private temporary directory and uses fresh state with one manual
test identity. It never starts Claude or Codex. Existing jobs and state are untouched.

macOS needs the current user's GUI launchd domain; Linux needs a reachable systemd
user manager. The Linux job is linked with `--runtime`, not enabled for login.
The test unregisters its job and removes its temporary files on success. On failure,
it attempts cleanup but retains its files and reports their path for inspection.
The manually triggered `Native supervisor lifecycle` workflow runs the same test
on both platforms. A missing supervisor is a failure, not evidence of a passing gate.
