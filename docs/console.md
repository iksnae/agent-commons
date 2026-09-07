# Terminal console

```sh
agent-commons console --config /absolute/private-role-connection.json
agent-commons console --config /absolute/private-role-connection.json --once
```

The console shows registered agents and tasks for that connection's project.
It refreshes every three seconds after the previous request finishes. Use arrow
keys to scroll and `q` to exit; the service keeps running. Redirected input or
output automatically selects a single JSON snapshot, as does `--once`.

This first view is read-only. It does not enroll or attach agents, acknowledge
messages, retry work, or fall back to operator credentials. Connection failures
mark the last successful snapshot as stale. A current attachment lease is not
proof an agent is reachable. Task acceptance is not evidence of deployment.

The interactive view displays up to 200 agents and 200 tasks, with bounded text
fields. It requires at least 40 columns and 12 rows. Messageboard browsing,
multi-project navigation and detailed provider/watcher health remain open.
Source bundles include vendored dependencies and their notices for offline rebuilds.
