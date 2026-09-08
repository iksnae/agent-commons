# Run as a user service

The `service` commands install and control one named launchd or systemd user job.
They do not enroll roles or supervise inbox watchers yet. The full command path
passed on macOS and Linux in [native CI run 34150065040](https://github.com/iksnae/agent-commons/actions/runs/34150065040):
installation, startup, crash recovery, restart and recoverable removal. Linux's
persistent startup links were also checked before and after removal. Login and
reboot recovery still need validation.

Use an installed binary and a separate private state directory. Stop any existing
foreground service using that state before starting the supervised one. Back up
state before switching versions. Do not use `sudo`.

## Install and start

Choose an existing configuration directory that is not group/world writable.
For login startup, use your macOS `Library/LaunchAgents` directory or Linux
`.config/systemd/user` directory. The command requires an absolute path and never
creates those directories for you. A macOS plist placed in LaunchAgents can load
at your next login even if you have not started it in this session.

```sh
agent-commons service install \
  --directory /absolute/user-service-directory \
  --binary /absolute/installed/agent-commons \
  --state /absolute/private-state \
  --path /absolute/provider-bin:/usr/bin:/bin
```

Use the returned `file` path for subsequent commands:

```sh
agent-commons service enable --file /absolute/returned-service-file
agent-commons service start --file /absolute/returned-service-file
agent-commons service status --file /absolute/returned-service-file
```

`enable` configures future user-session startup. `start` starts the job now;
starting an already loaded macOS job can fail. A successful command is not proof
that RPC or providers are ready. Check your role connection with `doctor`.
Linux startup depends on a working systemd user manager; this tool does not enable
lingering. macOS control requires a graphical login session.

Configuration and its `.json` receipt are mode 0600. Existing files are never
replaced. A failed install leaves evidence for inspection; it does not adopt an
existing matching service file. Receipts detect accidental changes, not malicious
changes by another process running as your user.

## Stop and remove

```sh
agent-commons service stop --file /absolute/returned-service-file
agent-commons service remove --file /absolute/returned-service-file --confirm-stopped
```

Confirm the service has stopped before removal. The confirmation flag is your
assertion; removal does not inspect processes. It disables the job and moves its
configuration and receipt into a private sibling directory. The output gives you
that recovery path. Agent state, messages and credentials stay where they were.
Changed configuration files are refused rather than overwritten or removed.

Supervisor failures can leave partially changed state. Inspect `service status`
and the reported retained path before retrying. A timeout does not mean an OS
command had no effect. To restore removed configuration, move both retained files
back to their original vacant paths, then explicitly enable and start the job.

Upgrade and automatic rollback are still open. Retain the old binary and a
matching state backup until migration has been tested.

## Known limit: the service's own stderr

`agent-commons init` starts the per-user service when none is running. It
captures that process's stderr to a private file in the state directory so a
failed startup can say why. On success the file is unlinked immediately, but the
running service keeps writing to it: the coordination service logs one line per
authenticated RPC (actor and method only — no parameters, content or
credentials).

The file has no directory entry while this happens, so it is invisible to `ls`
and `du`, occupies roughly 64 bytes per RPC — about 61 MiB per million calls —
and its storage is reclaimed in full when the service exits. If a long-lived
service makes that growth matter before it next restarts, restart it to reclaim
the space.

A supervised job has no unlinked file. The generated launchd plist and systemd
unit name no stderr destination, and `service install` and `service start` have
no flag that sets one.

On macOS the lines are written and discarded. launchd gives a job with no
`StandardErrorPath` `/dev/null` for standard error rather than routing it to the
unified log. Measured against a service these commands installed: fd 2 is
`/dev/null`, its byte offset climbing as the service serves RPCs, and
`log show --predicate 'process == "agent-commons"'` returns no rows for calls
made while it ran. A supervised macOS service therefore grows nothing and
retains nothing — no log to rotate, and none to read when you need to know what
the service was doing.

On Linux this has not been measured here. The unit names no `StandardError`,
where systemd's documented default is to inherit `StandardOutput` and journal
it, so expect that same line per RPC in journald under journald's retention.
Confirm it on the host before relying on either the volume or the record.

A service you start yourself with `serve` writes those lines to whatever stderr
you gave it.
