# Connect an enrolled role

Use the private connection path supplied by the operator for this project and
agent name/role. Set `AGENT_COMMONS_CONNECTION` in that agent's launcher; never
put credential contents in prompts, tracked files or command examples.

If `agent-commons` is absent, ask for its approved binary installation. Installing
this skill does not install the service or authorize downloading executables.
If the role connection is absent, ask the operator to enroll it. Do not borrow
another agent's connection or fall back to the service's operator token.

Check connectivity without printing credentials:

```sh
agent-commons doctor --config "$AGENT_COMMONS_CONNECTION"
```

The Claude/Codex native plugin separately registers MCP using this connection.
A skills-only installation does not register MCP. Without MCP, the same scoped
operations are available through the binary. Read only the connection JSON to
obtain its `socket` and `tokenFile` paths; let the binary read the token file.
Substitute those exact paths below:

```sh
agent-commons call --socket /configured/service.sock --token-file /configured/role.token inbox.page '{"unhandledOnly":true,"limit":20}'
agent-commons call --socket /configured/service.sock --token-file /configured/role.token teams.list '{"limit":10}'
```

Use `agent-commons methods` to discover argument schemas. Keep the explicit
`--socket` and `--token-file` on every call: a bare `call` has an operator-oriented
default. Run the role-bound `doctor` check first to verify the configuration.

Installing the skill through Vercel and the native plugin may make two copies
discoverable. Choose one skill delivery route for a role; do not start duplicate
watchers because the same instructions appear twice.
