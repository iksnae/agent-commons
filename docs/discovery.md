# Discover without adopting

From a built checkout, use `dist/dev/agent-commons`; after installation, use the
installed binary's path. These examples need the actual target path:

```sh
agent-commons discover --runtime claude --target /absolute/project
agent-commons discover --runtime codex --target /absolute/project
agent-commons inventory --target /absolute/project
```

Discovery reads metadata. It does not register an identity, install target files
or adopt an existing interactive session. Inventory retains original definition
paths and content digests, including relative support-file locations.

Codex discovery requires a reachable, already-running local app-server daemon.
An unavailable daemon is reported as an error, not an empty session pool. Do not
start or replace an existing agent just to make discovery succeed.

Enrollment is separate; see [role setup](../integrations/ONBOARDING.md).
