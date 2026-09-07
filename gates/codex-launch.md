# Codex launch integration

Problem: an installed plugin is not enough to make a Codex role check in on
launch. Hook discovery, operator trust, native session identity and durable role
attachment are separate steps. Silent failure at any step leaves the agent offline.

The adapter must verify these steps and report which one is missing. It must not
grant hook trust, attach an inherited lead credential to a fork, or acknowledge
messages on the agent's behalf.

## Verified interface

Checked on 2026-09-07 with local `codex-cli 0.153.4` and the
[official hooks documentation](https://learn.chatgpt.com/docs/hooks):

- Plugin hooks use the regular hook trust flow; enablement alone is insufficient.
- A SessionStart command receives JSON and can return concise additional context.
- The documented start sources do not distinguish forks. Do not port Claude's
  fork check by merely changing the runtime name.

The installed binary's generated app-server schema exposes `hooks/list` with
source path, current hash, enabled flag and trust status. The native test
`just codex-hook-test` confirms discovery of one isolated user-level SessionStart
hook as enabled and untrusted, with a nonempty hash. It starts a fresh stdio
app-server with a disposable Codex home and no inherited model credentials.
It does not connect to the user's daemon, trust a hook or start a model turn.

The same recipe also exercises native plugin install and uninstall in a fresh
Codex home. It verifies copied payload bytes, the MCP server name, the namespaced
skill and removal of the installed cache. `plugin/read` returns marketplace
source paths; the test uses `skills/list` to locate the installed skill instead.
The current Claude-only hook is excluded from Codex plugin metadata. A Codex
launch hook has not been added yet.

An earlier `codex debug prompt-input` probe returned JSON with no hook-review
diagnostic. Absence of hook text in that output did not establish that trust
prevented execution, so that probe is not acceptance evidence.

## Remaining acceptance checks

1. Load the packaged Codex hook through the real plugin loader without also
   running it in Claude. The existing dual-runtime package needs explicit routing.
2. Show the operator how to review the exact hook in `/hooks`. Verify untrusted,
   trusted and modified definitions through native execution, not inventory alone.
3. Establish a native identity binding that rejects unintended forks and child
   agents. The documented hook payload alone does not yet prove this property.
4. Verify startup and resume preserve the enrolled project/role, deliver guidance
   to model context, and leave messages unread until the agent reads them.
5. Exercise lease renewal, disconnect, service restart and inbox recovery.

Keep credential isolation separate: a hook check cannot prevent another process
that already has the same role token from using it directly.
