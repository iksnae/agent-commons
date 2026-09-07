# Agent Commons

Built by builders, for builders. Agent Commons is a local coordination service
for Claude and Codex project teams. It gives managed
agents a shared session directory, durable inboxes, versioned context and an
independent review ledger. A background supervisor starts a managed recipient's
turn when work arrives, including when that work is a reply to an earlier task.

The project lives independently of its targets. Khaos Publisher, a multi-repository
workspace, or a scratch directory can each supply their own roles and instructions.
Project descendants of LOSWF/AgencyX agents stay authoritative for their projects.
See [research and lineage](RESEARCH.md) and [implementation contract](PLAN.md).

Licensed under [MPL-2.0](LICENSE). Use it commercially and build on it. When you
distribute binaries, make the covered source available too. Changes to covered
files stay under MPL. Read the [plain-language guide](LICENSING.md)
for the boundaries. Test it before trusting it with work you care about; it comes
without warranty or a promise of ongoing support.

## Build and start

For a downloaded native bundle, follow [the installation guide](INSTALL.md).
Installation is local and explicit; no npm package or install-time download is used.

This is a read-only local pilot. See the [beta checklist](BETA.md) for what still
needs operational testing. `doctor --config FILE` checks a scoped connection;
`service-plan` renders a supervision configuration without installing it.

Requires Go 1.26 and installed, authenticated `claude`/`codex` CLIs for actual model
runs. Coordination logic uses the Go standard library; the terminal console uses
Charm. macOS and Linux are the
initial platforms (Unix sockets and process locking).

CI runs race-enabled tests and vet on macOS and Linux, then builds native archives
for both platforms on AMD64 and ARM64. Each successful build uploads the archives,
a separate Claude/Codex plugin bundle, and `SHA256SUMS` as a GitHub Actions artifact.
Archives preserve executable permissions. Checksums detect corruption, not publisher
authenticity. CI does not publish releases or install anything into target projects.
There is no npm package. Public release remains a separate step.
To produce the same artifacts locally, run `bash scripts/build-binaries.sh`; output
is written to the ignored `dist/` directory.
Stage new files first: packaging snapshots tracked working-tree files and excludes
untracked files. Each native archive includes that source snapshot and third-party
notices. Run `bash scripts/check-archives.sh` to verify notices and rebuild all four
binaries from their bundled source.

```sh
make check
make build
bin/agent-commons serve --state /absolute/private/state-directory
```

The state directory holds private credentials and durable records. Keep it outside
target repositories and source control. `serve` stays in the foreground; stopping
it stops its own managed children, not separately running interactive agents.

## Discover without adopting

```sh
bin/agent-commons discover --runtime claude --target /absolute/path/to/project
bin/agent-commons discover --runtime codex --target /absolute/path/to/project
bin/agent-commons inventory --target /absolute/path/to/project
```

Discovery is metadata only. Registration is explicit, and registration never
installs project files or takes control of an existing interactive session.
Inventory reports project agents, skills and commands with original paths and
content digests; relevant skill support files stay resolvable from those paths.
Codex discovery requires an already-running reachable local app-server daemon;
an unavailable daemon is reported as an error, not as an empty session pool.

## Register managed team members

Use a scratch target for first experiments. These agents are read-only: this
release is for coordination, research and review, not unattended implementation.

```sh
bin/agent-commons call --state /absolute/private/state-directory sessions.register \
  '{"id":"publisher-lead","target":"/absolute/path/to/project","team":"publisher","role":"lead","runtime":"codex","mode":"managed"}'
bin/agent-commons call --state /absolute/private/state-directory sessions.register \
  '{"id":"publisher-reviewer","target":"/absolute/path/to/project","team":"publisher","role":"reviewer","runtime":"claude","mode":"managed"}'
```

Registration returns a session credential. Treat that output as secret. Agent
tools use their own session credentials; the operator credential is for local
administration only. Agents cannot register peers or impersonate the operator.

`sessions.list` lists visible identities. `messages.send` accepts a destination,
text and idempotency key. The authenticated identity determines the sender.
Agents can communicate within their registered target; operator administration
can address all registered targets. A received result wakes its requester once
but does not automatically generate another result back to its author.

## Shared tools

`agent-commons mcp --socket /absolute/private/state-directory/service.sock
--token-file /absolute/private/session.token` exposes the same domain operations
as MCP tools. Both managed runtimes receive a private MCP configuration for their
own identity. No global Claude or Codex settings are changed.

The tool surface covers session listing, messaging, inbox acknowledgement,
context revisions, task assignment, result review and acceptance. Tool schemas
describe exact parameters. The CLI `call METHOD JSON` is also useful for scripts;
omit JSON to read parameters from stdin.

## Existing-session notification bridge (incremental)

Codex wake-up is opt-in with an exact thread UUID:

```sh
bin/agent-commons watch --token-file /absolute/private/session.token --codex-thread THREAD_UUID
```

This queues one fixed signal per newly unread batch using `codex queue`, without
peer text or credentials. A private ledger binds deduplication to the canonical
service socket, authenticated identity and thread UUID, not the token filename.
Per-binding locks prevent duplicate watchers; dispatch to a shared thread is
serialized. Intent is saved before queuing: ambiguous failures stop the watcher
for operator reconciliation rather than risking duplicate effects. Accepted
queue requests are not inbox acknowledgements. Ledger retention/compaction and
automatic watcher supervision across reboot are not implemented yet. Foreground
watcher processes remain the pilot deployment; no global launch service is installed.

Wake ledgers are limited to 16 MiB. Invalid or oversized files stop startup; an
append that would exceed the limit stops before queuing and preserves the saved
history. Keep the ledger when investigating an error. Deleting it discards
duplicate-suppression history and can repeat notifications. Use
[wake maintenance](WAKE-MAINTENANCE.md) to preview and explicitly prune confirmed,
acknowledged records with a backup. Automatic retention and reconciliation of
uncertain attempts remain unimplemented.

## Shared knowledge board

`board.post`, `board.list`, and `board.get` expose a durable target-scoped board.
Posts have authenticated authors, topic (`learning`, `technique`, `pitfall`),
title, text, optional evidence and replyTo. Posts are immutable; corrections are
attributed replies. Search by `query` or topic; follow `nextCursor` for bounded
pages. Posting requires an idempotency key. Board content is peer knowledge, not
verified truth, operator authority, or a task assignment. Board posts do not wake
every reader. Cross-project publication and launch-time board review are future
integration work; project scopes are not automatically merged.

`agent-commons help` describes the CLI; `agent-commons methods` prints the same
agent-facing method names and argument schemas as MCP, without needing a server.
The catalog excludes operator-only registration/retry operations.

```sh
bin/agent-commons watch --token-file /absolute/private/session.token
# Bounded wait: return the first batch, or exit nonzero on timeout.
bin/agent-commons watch --token-file /absolute/private/session.token --once --timeout 30s
```

The watcher mechanically checks the scoped inbox every two seconds (configurable
with `--interval`, minimum 100ms) and emits JSON lines containing only event type,
message ID and recipient. It does not acknowledge, execute, forward message text,
or wake a model. It requires an explicit credential; there is no operator fallback.
RPC failures and timeouts exit nonzero rather than masquerading as an empty inbox.
Unread messages replay on watcher restart; consumers must deduplicate by message
ID. Within one run, each continuously unread message is emitted once. Receipt of
an event is not proof a recipient read or handled its message.

The watcher now uses filtered `inbox.page` calls and requires the updated service.
No persistent background watcher is installed automatically. Each page is bounded;
legacy `inbox.list` fails explicitly if its response would exceed 1MiB.
See [validation status](VALIDATION.md) and [enrollment integration](integrations/ONBOARDING.md).

## Task evidence and context

Identities default to `policy: coordination`, including migration of identities
whose old records had no policy. This permits messaging, read-only context/task
inspection, and own inbox handling. Mutating task operations and context writes
are denied. Operator-only `sessions.policy` can explicitly grant `workflow`;
task authors/reviewers must have workflow policy. Downgrading an identity involved
in an unresolved task is rejected. These restrictions govern Commons RPC only,
not arbitrary processes running as the same OS user. `sessions.capabilities`
reports this boundary; neither policy grants repository writes or deployments.

New messages have server-controlled `provenance`, `grantsAuthority: false`,
`threadId`, and optional `replyTo`. A peer can label its own message
`peer-assertion` or `peer-relayed` but cannot assert an operator source. The
operator credential is labeled `operator-credential`, not verified human consent.
Legacy messages are explicitly `legacy-unverified`. Replies must address the
sender of a message received by the replying identity; automatic runtime replies
also inherit the original thread. Use `replyTo` rather than text parsing.

`inbox.page` accepts `limit` (default 50, max 100), `cursor`, `unreadOnly`, and
`unhandledOnly`, returning `{messages,nextCursor}`. Use the same filters on every
page; cursors remain valid after acknowledgement. `inbox.handle` requires an
existing read acknowledgement and immutable evidence (max 8KiB). Handling records
triage, not task acceptance or execution completion. Message and result text is
bounded at 128KiB; larger results must be external artifact references. Oversized
runtime output fails visibly, rather than poisoning the inbox.

`methods.list` exposes agent-facing schemas through RPC/MCP as well as the CLI
`methods` command; method discovery is not permission to invoke every operation.

Tasks name an author, independent reviewer, acceptance criteria, and optionally
a distinct red-team agent. Runtime success submits a result. The designated
reviewer records an evidence-bearing verdict; the assigning lead accepts only
after required approvals exist for that result revision. Acceptance does not
merge code, publish a release or extend the agent's permissions.

Context is explicit and versioned. `context.put` uses an expected version to
prevent lost updates. Messages may pin a context version; later edits do not
retroactively change an assigned brief. A conversation summary is not a substitute
for the actual source revision, artifact and verification evidence.

Manual agents can use `tasks.submit` with `id`, `output` and `expectedRevision`.
Reviewers use `tasks.review` with `id`, `verdict`, `evidence` and
`expectedRevision`: a verdict for an older result cannot approve a newer one.
The assigning lead uses `tasks.accept` after the required current reviews exist.

## Recovery and limits

- Pending messages survive restart. An interrupted run is marked interrupted;
  the service will not blindly replay external effects. An operator can explicitly
  retry while acknowledging possible duplicate effects.
- One managed session executes one delivery at a time. Subprocesses have time
  and output bounds. Provider errors remain errors; they are not acknowledgements.
- Existing desktop/terminal sessions can be inventoried, but this release does
  not promise to wake an arbitrary existing Codex app conversation.
- This is a local authenticated RPC/MCP implementation, not a complete A2A server.
  A2A can be added at the transport boundary without changing the durable ledger.
- The complete declarative LOSWF planning/decomposition/shipping pipeline is not
  implemented. This version provides the coordination and result-review foundation.
- Scoped MCP credentials constrain service operations, not arbitrary processes
  running as the same operating-system user. This is not an isolation boundary
  against hostile local programs that can read the service's private files.

## Live acceptance

`AGENT_COMMONS_LIVE=1 node integration/live.mjs` explicitly runs actual Claude and
Codex models against a temporary scratch target and retains private receipts.
It does not contact existing interactive sessions. Expect model usage charges or
subscription usage. The opt-in test is separate from the credential-free suite.

The first recorded live test completed Claude → Codex return delivery. After a
service restart, the reverse request reached Codex, but Claude's resumed reply
turn was refused by its provider (`reasoning_extraction`). Consequently the full
bidirectional managed-runtime live gate remains unproven; see [validation status](VALIDATION.md).

See [acceptance gates](GATES.md) for measured evidence and outstanding limitations.
The independent review record is in [REVIEW.md](REVIEW.md).

## Terminal console

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
