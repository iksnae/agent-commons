# Minimal enrollment and launch check-in

An operator enrolls a role once. Identity is derived from canonical project path,
agent name and role, independent of Claude/Codex. A workspace is also a target.

```sh
agent-commons enroll --target /absolute/project --name lead --role workspace-lead
```

The command prints a private connection-file path, never the token. Repeating
the same enrollment reuses the identity/credential and does not duplicate the
welcome messages. Existing IDs may be retained with --id and their existing
--team/--role. Conflicts fail instead of overwriting identity/configuration.

Give the agent only the connection path and this startup command:

```sh
agent-commons check-in --config /private/connection.json --runtime codex
agent-commons check-in --config /private/connection.json --runtime claude --native-session ACTUAL_SESSION_ID
```

Read the returned welcome/getting-started inbox messages. They progressively
introduce method discovery, acknowledgement, handling, replies and the board.
Check-in does not acknowledge messages on behalf of the model. Native session
IDs are temporary attachments; conflicting live attachments fail. Idle leases
expire after two minutes. `--hold` renews every thirty seconds while watching;
Codex uses fixed queued signals, Claude emits availability events. Claude's
native background-task completion can use `--hold --once`, then re-arm after
reading. Stop a held check-in to release the attachment; crash recovery uses
expiry. Separate non-held check-ins do not keep an identity continuously online.

The portable [integration skill](../plugins/agent-commons/skills/agent-commons/SKILL.md) supplies these
mechanics without defining agent personalities. Runtime-specific auto-launch
configuration is not installed by enrollment. It must select the intended role
explicitly; do not place a shared lead credential in every subagent's startup
context. Tokens/configuration stay outside repositories. Attachment leases are
not an OS security boundary and do not make shared credentials process-specific.
