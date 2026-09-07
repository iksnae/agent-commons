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
Busy bool. JSON fields camelCase. Credentials are omitted from every public DTO.
Delivery fields: ID, From, To, Text, Kind (`message`, `task`, `result`), TaskID,
ContextID, ContextVersion int, Status, Attempts int, Output, Error, CreatedAt.
Session and Delivery exported Go fields named exactly above.

Pi and Hermes support explicit manual attachment and shared coordination. Managed
registration remains limited to Claude and Codex until native adapters are tested.
See HARNESS-SUPPORT.md for the capability boundary and project inventory sources.

Core RPC methods with JSON params:
- `sessions.register`: operator-only; Session fields; returns session + token.
- `sessions.list`: operator all; agent same target only.
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
- `messages.retry`: operator-only {messageId}; failed/interrupted only,
  explicit duplicate-effect acknowledgement {acknowledgeDuplicateRisk:true}.

`Claim` is supervisor-only: atomically takes oldest pending delivery for one
managed session; one in flight per session. `Finish` atomically records output,
updates runtime session ID, and creates one result message addressed to original
sender. Result messages do not auto-reply, preventing endless acknowledgement loops.
Successful task run means `submitted`, not `accepted`. Failures preserve error and
never masquerade as results. Startup marks abandoned in-flight delivery interrupted;
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
