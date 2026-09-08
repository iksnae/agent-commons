# Claude launch check-in

The hook resolves the role connection from `AGENT_COMMONS_CONNECTION` when the
launcher sets it, and otherwise searches upward from the launch directory for
`.agent-commons/project.json`, so a launch inside an initialised project needs no
launcher environment at all. It requires Claude Code 2.1.214 or newer, a launch
from the enrolled project/workspace root or any directory beneath it, and an
already enrolled role. A sibling directory that merely shares a textual prefix
with that root is not beneath it and is refused. For `claude --agent NAME`,
also set `AGENT_COMMONS_CLAUDE_AGENT=NAME`. If the launcher uses a Claude binary other than
the one on PATH, set `AGENT_COMMONS_CLAUDE_BINARY` to its absolute path.

The hook runs `AGENT_COMMONS_BINARY` when the launcher sets it, otherwise
`$CLAUDE_PLUGIN_ROOT/../../agent-commons`, where `bundle install` puts the binary
relative to the plugin, and otherwise `agent-commons` on PATH, which is where a
source checkout keeps it. `bundle install` adds nothing to PATH, so the middle
step is the one an installed plugin normally uses. The bundled
`scripts/check-in.sh` shares the first two steps but, being an explicit command
rather than a launch hook, reports the failure instead of exiting quietly.

The hook is silent when there is no Agent Commons project to check in to: no
`AGENT_COMMONS_CONNECTION`, and no `.agent-commons/project.json` above the launch
directory. A plugin installed at user scope starts in every repository on the
machine and almost none of them are enrolled, so a notice in each one would be
noise rather than a diagnosis. The same silence covers a plugin installed with no
binary beside it. It is not silent about configuration that exists and is broken:
an initialised project with no enrolled role, a connection file that will not
open, a launch outside the enrolled root, or `AGENT_COMMONS_CONNECTION` set with
no binary findable each report on stderr and exit non-zero. Claude shows that in
the transcript and the launch continues. The hook never exits 2. On SessionStart
that code is not special — it shows stderr and continues, exactly like the 127
the hook actually uses — but 2 is the hook protocol's reserved signal for
blocking an action on the events that can be blocked. This hook has no business
sending that signal, and a reader who knows the protocol should not have to
check the event table to be sure a failed check-in cannot stop a launch.

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
