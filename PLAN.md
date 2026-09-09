# Agent Commons implementation contract

Independent, local-first coordination of Claude and Codex project teams. Target
repositories are registrations, never modified by onboarding. Project-owned agent,
skill and command definitions take precedence over canonical ancestors. Runtime
processes are disposable; messages, task evidence and context are durable.

## Scope and acceptance

Build a Go standard-library service, CLI and MCP stdio tool interface, durable
session registry and inbox, task/review ledger, versioned shared context, local
definition inventory, Claude/Codex CLI adapters, and automatic managed-session
wakeup. Existing sessions can be discovered but are not adopted or steered merely
because they were found. Use explicit registered managed agents for live tests.
No GitHub publishing, production deployment, target edits, or global configuration.
No claim of complete A2A protocol compliance: the initial transport is a local
authenticated RPC/MCP boundary with an A2A mapping documented separately.

## Team participation and later cross-team learning

Delivery priority: Claude and Codex are the core working-team harnesses. Complete
their launch/resume, delegation, durable return paths and reviewed project work
before expanding secondary harnesses. Hermes participates as a visitor; its
native acceptance work is deferred, not a prerequisite for the initial core-team
release. Pi support remains in scope behind core-team delivery. See PRODUCTION.md
for the operator-confirmed priority and the acceptance boundaries still open.

Commons is the campus: a shared place where agents can connect and exchange
knowledge beyond an individual task or project. Teams are work rooms within it,
not the outer boundary of the product. Campus-wide social and learning spaces
remain planned; the current service enforces project scopes.

Harness support is meant to let an agent join a team session. Runtime launch and
notification adapters support that workflow; they are not the product outcome.
The coordination team session is distinct from each participant's native model
session or transcript. Project + agent name/role remains the stable identity;
native sessions are replaceable attachments, and team membership is a separate
relationship rather than a copied native conversation.

The next participation workflow must let an authorized Hermes, Pi, Claude or
Codex agent join the intended team, receive its brief and relevant learnings,
read its inbox, and take part in task handoff, review and return paths. Leaving
or reconnecting must preserve messages and evidence without silently broadening
authority. Existing runtime check-in and the `Team` label do not by themselves
prove this complete membership workflow.

The first membership slice now provides operator-created immutable team briefs,
explicit same-project invitations and revocation, and agent get/join/leave/rejoin.
It persists separately from native attachments and the legacy `Team` label.
See HARNESS-SUPPORT.md for membership usage. Optional team-scoped task, context
and result routing now uses schema 3; [team work](docs/team-work.md) describes
its access and upgrade boundaries. Native participation, knowledge review and
isolated execution still need integration and acceptance.

A later system scope will support persistent boards and inboxes beyond project
work: shared learnings, techniques, strategies, ideas and experiments across
teams. Publication or membership must be explicit. Project credentials, private
messages and work authority must not become system-wide merely because their
author joins a shared space. Shared knowledge remains attributed peer data;
publishing an idea does not authorize an experiment. This is planned scope, not
an existing cross-project access grant or implemented feature.

Hermes can later serve as a system-level orchestrator and organizer across
workspaces and projects. Its visitor status describes the current integration
priority, not a permanent limit on its role. That future role can coordinate
project leads, organize shared learnings and track work across explicitly
connected workspaces. Claude/Codex project teams remain responsible for their
project-shaped execution and review workflows.

System-level coordination needs its own explicit identity and access scope; it
must not reuse a project's role credential as universal authority. Workspace and
project participation must be granted and revocable. Private inboxes, target
definitions and work permissions stay within their existing boundaries unless
explicitly shared or delegated. This is future product direction, not permission
to enroll Hermes, adopt sessions or open cross-project access now.

## Planned scope from an external harness review

Four concepts were taken from a review of a third-party agent harness and its
adjacent RL rollout body; RESEARCH.md pins both sources and records what was
rejected. They are recorded here as planned scope so a later sequencing decision
has something concrete behind it. None is approved, ordered or assigned, and each
names what it would cost here and what it collides with. Every statement below
about what Commons already enables sits downstream of G5, which remains open.

Typed causal edges publish an explicit relationship between request identifiers:
`continuation`, `subagent_call`, `subagent_return`, `compaction`, carried as a
namespaced extension that unaware consumers ignore. `Delivery` today carries
`TaskID`, `ContextID`, `ContextVersion` and `Status` but no edge type, so causal
relationships between deliveries are reconstructed by convention. The status log
below records the failure this would have made legible: the reverse request that
reached Codex after restart, and the resumed Claude acknowledgement refused with
`[reasoning_extraction]`. Collision: the interface freeze below fixes `Delivery`'s
exported fields, so this is a schema migration with a pre-upgrade snapshot on
the pattern schema 3 already set, not an edit to the frozen list. It takes the
next free schema number at the time it merges; schema 4 is already taken by task
abandonment and schema 5 by session retirement, and numbers are never shared
between features.

A capability broker keeps credentials in a supervisor and gives the child only an
opaque capability and a socket path, so the child never sees the secret.
PRODUCTION.md names launcher and role credential isolation as open in three
separate places, and the current mitigation filters the managed environment rather
than withholding the credential. Collision: the existing bearer token is per-actor
and durable, while a capability is per-invocation and disposable. Issue, bind,
expire and revoke are new lifecycle state in `internal/core`, the package that owns
all persistence and domain transitions.

Idempotency attempts as a distinct dimension means one stable key identifies a
logical call across every retry layer while a separate counter distinguishes
attempts, with both header names reserved against user configuration.
`messages.send` and `tasks.assign` already accept `idempotencyKey`, `Delivery`
already carries `Attempts`, and `messages.retry` already demands explicit
duplicate-effect acknowledgement. This may therefore be a semantics clarification
of the existing `Attempts` field rather than new state. Which of the two it is has
not been established; a reviewer should settle that before anyone builds it.

Strict contract completeness sends every field explicitly including disabled ones,
rejects partial or unknown configurations, and never falls back to the ambient
process environment. Commons already holds this stance in parts through
`sessions.capabilities`, and through HARNESS-SUPPORT.md on an advertised service
that fails discovery returning an error instead of silently falling back. The
addition is the completeness requirement itself. It addresses a finding already
recorded in PRODUCTION.md: the standalone app-server starter loads native
user/project configuration, unlike the managed execution policy. Collision: the
same finding records that Codex 0.153.4 rejects app-server `--ignore-user-config`
and `--ignore-rules`, so completeness cannot be obtained from that flag surface
alone.

## Shared interfaces (freeze before parallel work)

Module `agentcommons`; Go 1.26. The original dependency-free constraint was amended
by operator approval for Charm in the terminal presentation layer. Coordination
core and service contracts remain standard-library-only. Package `internal/core`
owns all persistence and domain transitions. Public API:

```
New(directory string) (*Service, error)
(*Service).Close() error
(*Service).Call(actor, method string, params json.RawMessage) (any, error)
(*Service).Authenticate(token string) (string, bool)
(*Service).Token(actor string) (string, error)
(*Service).Claim(agentID string) (*Delivery, error)
(*Service).Finish(deliveryID, runtimeSessionID, output string, runErr error) error
(*Service).Sessions() []Session
```

`operator` is reserved. Token returns internal credentials, never exposed by RPC.
Session fields: ID, Target (absolute directory), Team, Role, Runtime (`claude`,
`codex`, `pi`, `hermes`, `manual`), Mode (`managed`, `manual`), RuntimeSessionID, Instructions,
Busy bool, RetiredAt, RetiredReason, Retirements. JSON fields camelCase.
Credentials are omitted from every public DTO. RetiredAt, RetiredReason and
Retirements are the session retirement amendment to this frozen list: a
deliberate contract change, not an incidental one. RetiredAt is an RFC3339Nano
timestamp rather than a bool because the audit trail is the reason retirement
tombstones instead of deleting. Retirements is an append-only `[]Retirement`
(`At`, `Reason`, `ReinstatedAt`, `ReinstatedReason`) recording every withdrawal
and return; RetiredAt/RetiredReason are current state and every rule reads
those, while the ledger decides nothing, the same split Task draws between
Status/Output/Revision and Reviews. It exists because reinstatement clears the
current-state fields, and a contract reading "nothing is erased" cannot have
reinstatement erase the retirement. Retirements is OPERATOR-ONLY: it is stripped
from every Session handed to any other actor, and from `Sessions()`, through the
single `core.sessionView` helper, because a reinstated identity is listed to its
peers again. `omitempty` keeps the `[]Session` wire shape unchanged for clients
that never see it. Both retirement evidence bounds govern stored bytes.
The service is the ledger's SOLE author: `sessions.register` and
`sessions.enroll` refuse a client-supplied `RetiredAt`, `RetiredReason` or
`Retirements` together. That refusal is what makes the ledger evidence rather
than input, on the same footing as `Review.Evidence`, `Delivery.HandlingEvidence`
and `Task.AbandonEvidence`, none of which is settable at registration. It also
keeps a legal call from writing state `validateRetirement` would refuse at every
later load, which no supported operation could then repair. Underneath those
refusals, and independent of them, both handlers build the stored `Session` from
a named field list rather than copying `params` wholesale, so a field added to
`Session` is not client-settable until it is deliberately added to that list.
Delivery fields: ID, From, To, Text, Kind (`message`, `task`, `result`), TaskID,
ContextID, ContextVersion int, Status, Attempts int, Output, Error, CreatedAt.
Session and Delivery exported Go fields named exactly above.

Pi and Hermes support explicit manual attachment and shared coordination. Managed
registration remains limited to Claude and Codex until native adapters are tested.
See HARNESS-SUPPORT.md for the capability boundary and project inventory sources.

Core RPC methods with JSON params:
- `sessions.register`: operator-only; Session fields; returns session + token.
- `sessions.list`: operator all; agent same target only. Retired identities are
  excluded by default; operator-only `{includeRetired:true}` returns them too,
  so a tombstone is never invisible.
- `sessions.retire`: operator-only {id,evidence}; evidence nonempty after
  trimming and at most 8KiB, matching `tasks.abandon`, `tasks.review` and
  `inbox.handle`. Tombstones an identity in place: the `Session` record stays
  with Name, Role and Target intact, `RetiredAt`/`RetiredReason` are set, and
  the credential is deleted from the token map. That deletion is the whole
  withdrawal — `Authenticate` iterates the token map and `Call` rejects any
  actor absent from it. The record is deliberately not deleted: `deliveryTarget`
  resolves a delivery's project through `Sessions[d.From]`/`[d.To]`, so a
  deleted record yields `Target == ""` and `deliveryAccess` then hides every
  team-scoped delivery that identity ever sent or received, including from the
  operator's own `inbox.list`. `BoardPost.Author` and `Review.Actor` are bare
  identity strings with no denormalized name and stay resolvable for the same
  reason. There is no delete, no purge and no force flag.
  Dispositions: inbox preserved unchanged including `Acknowledged`/`Handled`;
  undelivered (`pending`/`acknowledged`) deliveries to it marked `interrupted`
  with `Error: "recipient identity retired"`, never `completed`, because
  delivered does not mean read; board posts immutable with `Author` unchanged;
  `Context` untouched, having no author field; review verdicts retained verbatim
  in `Task.Reviews` with the retired actor's ID, and the acceptance predicate
  unchanged; team memberships set to the existing `revoked` value rather than
  removed; the attachment zeroed to the terminal value `sessions.detach` writes.
  Three hard refusals: a busy identity, an identity holding a live attachment
  lease (`Attachment.ExpiresAt > now`; the lease is 120s, so the remedy is to
  wait or `sessions.detach`), and an identity in `Lead`, `Author`, `Reviewer` or
  `RedTeam` of a non-terminal task — that last names the blocking task IDs and
  is the review-bypass boundary, since retiring a designated reviewer must never
  dissolve a review obligation. Non-terminal means neither `accepted` nor
  `abandoned`, through `core.terminalTaskStatus`. Operator only: an agent may
  not retire itself or anyone. Mechanically self-retirement destroys the
  caller's credential mid-call, and structurally it is review evasion. A retired
  identity is refused as a `messages.send` recipient and as a `tasks.assign`
  author, reviewer or redTeam; `Claim` yields it nothing; `messages.retry` will
  not re-queue a delivery addressed to it.
  The first successful retirement advances the state schema to 5 after taking a
  pre-migration snapshot; see the schema note below.
- `sessions.enroll` refuses to resurrect. The server resolves enrollment by
  scanning target + role + (name empty or matching) and overrides the client's
  derived ID with what it finds, so a re-`init` lands on the retired record
  whatever the client guessed. Retired candidates are classified separately from
  live ones: exactly one live candidate is adopted and any retired ones ignored;
  zero live and at least one retired fails, naming the retired ID, its
  `RetiredAt`, its `RetiredReason` and the two ways forward; more than one live
  gives the existing ambiguity error, unchanged. Silently adopting would restore
  a live credential with no operator decision, and minting a second identity
  would violate the target/name/role uniqueness registration already enforces.
- `sessions.reinstate`: operator-only {id,evidence}, the inverse of retirement
  and a separate deliberate act. Same ID, a NEW credential — the old one was
  destroyed and does not come back — `RetiredAt`/`RetiredReason` cleared, inbox
  returned intact with its acknowledgement flags as they were, board posts and
  task history untouched. Team memberships stay `revoked`: re-invitation is its
  own act. Onboarding messages are not re-sent, because `welcome` is idempotent
  on its onboarding key. It closes the open `Retirements` entry with its own
  timestamp and evidence before clearing current state, so the evidence it
  demands is persisted like every other evidence-bearing method here rather than
  demanded and discarded. `validateRetirement` refuses to load state whose
  ledger and current state disagree: a retired identity must have exactly one
  open entry matching its `RetiredAt`/`RetiredReason`, and a live one none.
  Credential lifecycle stays out of scope. `sessions.reinstate` returns the new
  credential in its response, so `agent-commons call sessions.reinstate` yields
  it; the `reinstate` subcommand deliberately does not print it and writes no
  file, so the enrollment credential file stays stale and the operator replaces
  it by hand or the identity cannot connect. Where that boundary belongs is an
  open decision, recorded in gates/core.md and docs/coordination.md.
- Durable schema 5 is taken lazily, on the first successful retirement only. A
  refused retirement writes no snapshot and advances no number. The bump is what
  makes a downgrade safe: an older binary has no notion of `RetiredAt`, so it
  would list a withdrawn identity as active, let `tasks.assign` name it and let
  `sessions.enroll` adopt it. Loading refuses a schema-5 state in which any
  session carries `RetiredAt` while still holding a credential, so credential
  destruction is durable state rather than only a code path.
- `messages.send`: {to,text,idempotencyKey,contextId?,contextVersion?}; sender
  derived from actor; operators can send to any target, agents only same target.
- `inbox.list`: actor inbox, operator {sessionId}.
- `inbox.acknowledge`: {messageId}; only receiver (operator may specify sessionId).
- `context.put`: {id,text,expectedVersion}; target is actor target, operator
  supplies {target}; version mismatch rejected. Immutable versions retained.
- `context.get`: {id,version?,target?}; enforce target boundary.
- `tasks.assign`: {to,title,text,criteria,reviewer,redTeam?,idempotencyKey,
  contextId?,contextVersion?}; reviewer and optional redTeam distinct from author,
  all same target. Queues task delivery and records assigning lead identity.
- `tasks.get`: {id}; scoped by target.
- `tasks.list`: scoped by target (operator all).
- `tasks.review`: {id,verdict,evidence}; actor must designated reviewer/redTeam,
  verdict `approved` or `rejected`, evidence nonempty. Only after submitted result;
  immutable verdict bound to current result revision. No self-review.
- Contract correction: `tasks.review` also requires `expectedRevision`, checked
  atomically against the submitted result. `tasks.submit` accepts
  `{id,output,expectedRevision}` from the author and advances the result revision;
  previous verdicts remain history and cannot approve the new revision.
- Delivery has an additional `Acknowledged bool` field independent of execution
  status, so reading an inbox never removes work from the managed execution queue.
- `tasks.accept`: {id}; assigning lead/operator only, requires submitted output,
  reviewer approval and redTeam approval if configured. Never means merged/shipped.
- `tasks.abandon`: operator-only {id,evidence}; evidence nonempty after trimming
  and at most 8KiB, matching `tasks.review` and `inbox.handle`. Sets status
  `abandoned` from any non-terminal status. It exists because a submitted result
  carrying a rejection at the current revision advances only when its author
  resubmits, so a task whose author will never act again pins every participant
  in an unresolved obligation with no supported exit.
  Retains `Output`, `Revision` and every `Reviews` entry unchanged, and records
  the operator's reason in the new `Task.AbandonEvidence` field, which is never a
  verdict and never joins `Reviews`. Enqueues nothing, is refused from `accepted`
  and from `abandoned`, and is not reversible: `tasks.submit`, `tasks.review` and
  `tasks.accept` all refuse an abandoned task, so no sequence reaches `accepted`
  through it. Abandonment records that work stopped, never that it passed.
  Unlike the other task methods it is not blocked when a team participant left,
  because it is the operator coordination that gate demands.
  The first successful abandonment advances the state schema to 4 after taking a
  pre-migration snapshot; see the schema note below.
- `accepted` and `abandoned` are the two terminal task statuses. Sites reasoning
  about terminality agree through `core.terminalTaskStatus`: `sessions.policy`
  refuses to restrict an identity participating in a non-terminal task, and
  `tasks.submit` refuses a terminal task. The acceptance predicate deliberately
  does not use that helper — it still tests `accepted` alone.
- No delivery carrying an abandoned task's ID may run, whatever its kind.
  `core.taskAbandoned` is the single predicate; `deliveryRunnable` and
  `messages.retry` both call it so the queue and the retry gate cannot drift.
  For task work this stops a claim moving the task back to `working`; for the
  result reporting it, this stops a managed session spending a paid runtime turn
  on terminated work. Acceptance is excluded from that predicate, because result
  deliveries carry the ID of tasks that legitimately reach `accepted`.
  Refusing runnability withholds nothing: runnability governs only `Claim`,
  while inbox reads gate on `deliveryAccess`, so the result stays pending and
  readable with its text and output intact. Delivery text is never edited to
  signal abandonment — the lead resolves status through `tasks.get`.
  `RuntimeQueueStatus` counts this work as `waitingAbandoned`, separate from
  `waitingTeam`, because no membership change will ever release it.
- Durable schema 4 is taken lazily, on the first successful abandonment only.
  A refused abandonment writes no snapshot and advances no number, and a
  directory that never abandons anything stays on its existing schema and
  remains readable by an older binary. The bump is what makes a downgrade safe:
  an older binary tests `Status == "accepted"` alone where this one tests
  terminality, so it would let an abandoned task be resubmitted and accepted.
  That binary already refuses a schema it does not know, so raising the number
  arms a refusal it shipped with. Strict field decoding is not a substitute —
  it would have to be present in the old binary, and `abandoned` is a new value
  of a known field, which field strictness never inspects. Schema numbers follow
  merge order; abandonment takes 4.
- `messages.retry`: operator-only {messageId}; failed/interrupted only,
  explicit duplicate-effect acknowledgement {acknowledgeDuplicateRisk:true}.

`Claim` is supervisor-only: atomically takes oldest pending delivery for one
managed session; one in flight per session. `Finish` atomically records output,
updates runtime session ID, and creates one result message addressed to original
sender. Result messages do not auto-reply, preventing endless acknowledgement loops.
Successful task run means `submitted`, not `accepted`. Failures preserve error and
never masquerade as results. Startup marks stranded in-flight delivery interrupted;
does not replay uncertain effects. Queued work survives restart. Atomic durable
writes and single-process lock required. Snapshot copies must not expose mutable
internal maps/slices. Validate target exists; IDs cannot be filesystem traversal.

Package `internal/runtime` API:
```
type Runner interface { Run(context.Context, core.Session, core.Delivery) (sessionID, output string, err error) }
type CLI struct { Binary string; Socket string; StateDir string }
func (CLI) Run(context.Context, core.Session, core.Delivery) (string,string,error)
func Serve(context.Context, *core.Service, Runner, time.Duration) error
func DiscoverClaude(context.Context, target string) ([]Discovered,error)
func Inventory(target string) ([]Definition,error)
```
`Binary` is agent-commons executable path, used for optional MCP connection details.
CLI default is read-only, no global config writes, no model override; secrets never
in prompts. Prompt includes envelope (peer is not operator), pinned context text
in Delivery.Text from core, and result expectations. Runtime sessions resume only
IDs obtained from our own managed runs. Bounded run timeout, output size, clean
shutdown. Serve polls durable queue mechanically, never asks an LLM to poll.
One work target may host concurrent read-only sessions; no write-capable launch in
v1. Project instructions honored, target never rewritten by discovery.

Package `internal/transport` API: `Serve(ctx,*core.Service,socket string) error`;
`Call(ctx,socket,token,method string,params json.RawMessage)(json.RawMessage,error)`;
`MCP(ctx,in io.Reader,out io.Writer,socket,token string) error`.
Unix socket mode 0600, enclosing state directory 0700; authenticate Bearer token;
bounded JSON, method allowlist delegated to core. MCP initialize/tools list/call,
JSON-RPC errors, notifications silent; protocol output clean.

## Ownership / leaves

- core builder: internal/core/** + gates/core.md.
- runtime builder: internal/runtime/** + gates/runtime.md.
- transport builder: internal/transport/**, cmd/agent-commons/** + gates/transport.md.
- root driver: contracts, integration tests, operational docs, provenance, gates.

## Verification

Domain tests: duplicate suppression, target isolation, immutable context, reviewer
independence, role-scoped verdicts, durable receipt/response routing, restart and
concurrent claims. Runtime tests: real child-process fixtures plus opt-in actual
Claude/Codex runs (fake success is not live evidence). Transport tests: auth, MCP
framing and errors, CLI smoke. Root reruns `go test -race ./...`, `go vet ./...`,
build and end-to-end restart/return-path tests. Independent adversarial reviewer
checks implementation before closeout.

## Status log

- Initial contract written; LOSWF lineage review in progress.
- Implemented core, runtime and transport leaves; root integration restart/reply test passes.
- Independent review found/fixed stale-review CAS, acknowledged-work loss, result
  restart task demotion, and retry reopening accepted work.
- Live Claude context-read → result → automatic Codex acknowledgement passed.
  Reverse request reached Codex after restart; resumed Claude acknowledgement
  was refused by its provider with `[reasoning_extraction]`. Not retried around.
