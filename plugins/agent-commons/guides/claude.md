# Claude launch check-in

The hook is inactive unless the launcher sets `AGENT_COMMONS_CONNECTION`. It
requires Claude Code 2.1.214 or newer, a matching project/workspace root, and an
already enrolled role. For `claude --agent NAME`, also set
`AGENT_COMMONS_CLAUDE_AGENT=NAME`. If the launcher uses a Claude binary other than
the one on PATH, set `AGENT_COMMONS_CLAUDE_BINARY` to its absolute path.

On startup or resume, the hook attaches the native session to that identity and
returns brief instructions to read its inbox and project learnings. It does not
read message bodies, acknowledge messages, start a watcher or create identities.
The attachment expires after 120 seconds unless renewed; use the skill's held
check-in flow for ongoing notifications. Errors do not grant fallback credentials.

Forks and subagent events are rejected by the hook. This is not credential
isolation: a child that inherits the connection environment or MCP server may
still use the same role credential. Do not distribute a lead's connection to
unrelated agents. Separate launcher/session credential boundaries remain work
in progress. Older Claude versions cannot reliably distinguish fork events;
see the [hook reference](https://code.claude.com/docs/en/hooks#sessionstart).

An isolated native `--init-only` test passed with Claude Code 2.1.236 on macOS.
It proved plugin loading, attachment and preservation of unread welcome messages,
without a model conversation. It does not prove a model read the injected guidance
or followed it, nor does it prove Codex startup behavior.
