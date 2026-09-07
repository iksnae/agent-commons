# Run as a user service

The `service` commands install and control one named launchd or systemd user job.
They do not enroll roles or supervise inbox watchers yet. This command path has
filesystem and fake-supervisor tests; its full native lifecycle and login/reboot
behavior still need validation. Existing native lifecycle CI exercises the lower
level service plans, not this installer.

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
