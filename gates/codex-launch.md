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
- The common hook input documentation says subagent hooks use the parent's
  `session_id`. A matching hook ID alone does not prove an independent identity.

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

## Native identity evidence

`TestNativeCodexDistinguishesResumedAndForkedThreadIdentity` passes on Codex
0.153.4. It starts a root in a disposable Codex home, adds fixed fixture history
through `thread/inject_items`, resumes it, forks it and reads the fork metadata.
Resume preserves the exact root thread ID and session ID. Fork returns a different
thread ID with `forkedFromId` naming the root; `thread/read` retains that ancestry.
Both are top-level threads with no `parentThreadId`. This does not test a spawned
subagent or equate `sessionId` with a unique thread ID.

The initial empty root could not be resumed: Codex reported that no rollout
existed. Adding fixture history made it resumable. A launcher must therefore
distinguish receiving a thread ID from establishing recoverable session history.
The test makes no model request, trusts no hook and attaches no Agent Commons role.

Use this evidence to build an explicit launcher binding: record the exact thread
created for a role, verify that thread's native metadata on resume, and treat forks
as separate enrollment decisions. A hook's first observed ID must not silently
claim the role. The launcher still needs a verified hook-to-thread handoff,
credential isolation and crash-recovery tests before automatic attachment is safe.

## Preparation implementation

`internal/codexlaunch.Prepare` now accepts an initialized native RPC connection,
a verified role scope and a checkpoint journal. The calling launcher must verify
the role credential and open the intended Codex home; this package cannot infer
either from an RPC stream.

Preparation saves an exclusive reservation before `thread/start`. It checks the
returned UUID, exact target and explicit empty fork/parent ancestry, then saves
the thread ID before adding fixed startup guidance. A matching `thread/read`
response permits the final ready checkpoint. No model turn or inbox operation
is sent. Read-only sandbox and no-approval policy are requested for the thread.

The directory journal writes private, exclusive files and syncs each checkpoint
and its directory. Existing reservations stop another preparation, including after
restart. Failed writes retain whatever evidence exists. An incomplete reservation
may correspond to a native thread whose ID was never saved; do not delete the
reservation and blindly repeat preparation. Inspection and operator recovery are
still needed for that case. A ready checkpoint is not a live attachment.

`TestNativeCodexPreparesRecoverableRoot` exercises this implementation against
Codex 0.153.4, stops that app-server, and resumes the saved thread through a fresh
app-server. It uses disposable state without model calls or hook trust. This
proves that this prepared history survives process replacement, not that an
arbitrary interrupted preparation can be recovered.

`internal/codexlaunch.LoadReady` reads the saved identity after restart. It
requires the caller's independently verified role, target and Codex home to match
the reservation exactly. Version 1 and matching created/ready thread IDs are
required. The reader pins the private binding directory while opening bounded,
private regular checkpoint files. Missing, malformed or conflicting evidence is
an error; reading never repairs files or releases the reservation.

The native preparation test now uses this reader before resuming through the
fresh app-server. File-boundary tests reject symlinks, FIFOs, public permissions,
oversized or trailing data, unsupported versions and changed scopes. Local
checkpoint validation does not prove native ancestry or grant attachment
authority. A launcher must still verify native metadata before using the role.

`internal/codexlaunch.Resume` performs that native identity check around resume.
It inspects the exact saved ID first and rejects changed targets, forks, child
threads or missing ancestry fields before requesting resume. The resume request
uses the same ID and target with read-only sandbox and no-approval settings. Its
response must still identify the same root. RPC failures stop without retry.

The native restart test now exercises this path. A separate native fork check
confirms rejection after inspection, before any resume request. Tests also cover
changed response metadata and cancellation between inspection and resume. These
checks prove identity handling, not full sandbox or credential isolation. The
caller still owns the scoped native connection; no role attachment or model turn
is performed by this package.

## Preparation command

```sh
agent-commons codex-prepare --config /private/connection.json --codex-home /private/codex-home
```

The command verifies the enrolled credential/target/name/role first. It requires
an explicit real private Codex home and records the binding under the configured
Agent Commons state directory. Native startup is deferred until the reservation
has been saved; an existing binding prevents another startup. The JSON report
includes the checkpoint directory and any known thread ID, including on failure.
Retain that directory if preparation is incomplete. Do not delete it to retry.

The command owns a separate stdio app-server process group, limits preparation to
45 seconds and closes it after preparation. It sends no model turn or role
attachment request and does not acknowledge the inbox. The native CLI test uses
a disposable home with an offline fixture provider.

The ordinary race suite also checks shutdown with a real subprocess fixture.
Cancellation during initialization and explicit close after initialization both
close a descendant's independent connection; repeated close is safe. Replacing
the process-group kill with a parent-only kill makes both tests fail because the
descendant remains connected. These fixtures exercise the launcher's shutdown
boundary without a native agent or model call. They do not cover descendants
that deliberately leave the owned process group.

The close assertion runs without a parent cancellation timer; startup alone has
a timeout. A no-op close also fails the test, so fallback cancellation cannot
stand in for the shutdown behavior being checked.

The selected Codex home and target still supply configuration. They may contain
credentials, MCP servers and plugins. The child environment allowlist is not
configuration isolation. Hooks are disabled for preparation, but other configured
native components may initialize, start subprocesses or use the network. Review
that configuration before using an existing home. No hook trust is granted.

This command prepares a thread; it does not launch an interactive role or complete
the hook handoff. Automatic check-in, attachment renewal, process-specific
credential isolation and incomplete-reservation recovery remain open.

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
