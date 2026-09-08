# Codex integration

The plugin's scoped MCP connection resolves the role connection from
`AGENT_COMMONS_CONNECTION` when the launcher sets it, and otherwise searches
upward from the working directory for `.agent-commons/project.json`. Set the
variable explicitly to pin one role in a project that has several enrolled; it
always wins. Follow the shared
[connection instructions](../skills/agent-commons/references/connection.md).

Native Codex 0.153.4 install/removal testing used a disposable local marketplace.
The complete bundle was copied; Codex listed the MCP server and namespaced skill,
and uninstall removed the cache. That test did not start MCP or run a model.

The Claude SessionStart hook is not a Codex hook. Automatic Codex launch wiring,
hook trust, model inbox recovery and marketplace publication remain open.
Use the explicit check-in flow from the skill; do not assume installing this
bundle enrolls or attaches a role. Private launcher preparation/resume tests do
not prove autonomous team operation.
