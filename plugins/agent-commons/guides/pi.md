# Pi package installation and startup

Use Pi's own installer with this extracted package directory:

```sh
pi install /absolute/path/to/agent-commons
```

Pi installs at user scope by default; add `-l` for project scope. Run
`pi remove /absolute/path/to/agent-commons` to remove that registration. Local
packages remain at their original path, so keep the extracted directory intact.
There are no runtime npm dependencies; `private: true` prevents npm publication.

The launcher must supply absolute `AGENT_COMMONS_BINARY` and
`AGENT_COMMONS_CONNECTION` paths for the intended enrolled role. Without both,
startup either stays inactive (neither supplied) or reports failed configuration.
Installing the package does not enroll a role or start the Commons service.

On startup, resume or reload, the extension reads Pi's native session ID and
header. It rejects fork ancestry and new/fork lifecycle events, then checks that
the native working directory matches the enrolled project before attaching.
It sends fixed getting-started guidance as a custom message, without triggering
a model turn. Inbox bodies, credentials and lease secrets are not injected.
The model uses the shared skill's CLI route; this package does not register MCP
tools in Pi or automatically join a team.

The attachment expires after 120 seconds unless held or renewed. Failures are
visible but not retried automatically; a timed-out check-in may have attached.
Inherited role credentials are still accessible to same-user child processes.
This is lifecycle integration, not credential isolation or a sandbox.

Native Pi 0.84.1 testing uses isolated configuration, offline startup and no
model tools. It verifies local installation, exact session attachment, unread
welcome messages, exact-file resume, fork rejection and removal. The resume
fixture starts with a native-format session header; no model output is invented.
Pi defers writing a fresh session until an assistant message, so a returned
session path alone does not prove persistence. Fresh-session recovery and actual
model participation remain separate acceptance work.
