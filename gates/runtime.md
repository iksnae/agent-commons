# Runtime leaf evidence

Owned scope: `internal/runtime/**` and this gate record.

## Pass 1 — contract and implementation

Read frozen PLAN.md and GATES.md. Inspected installed `claude --help`,
`claude agents --help`, `codex exec --help`, `codex exec resume --help`, and
`codex app-server proxy --help`; no model calls. Implemented managed-only CLI
execution, serialized durable claims via core, result completion, bounded timeout
and output, cancellation, project definition inventory and Claude discovery.

## Pass 2 — project shaping and return tools

Added MCP configuration with per-session credentials supplied internally by Serve.
Credentials live in per-run private temporary files, never prompt or argument
values; removed on exit. Added resolved provenance, SHA256, relative reference
base directories, native-runtime preferred role selection, explicit missing-role
notice, and cross-runtime instruction caveat. Added read-only existing-daemon
Codex thread discovery, without starting a daemon or adopting a thread.

## Pass 3 — adversarial corrections

Root review caught SKILL support markdown misclassification and nested directory
symlink omissions. Inventory now admits only SKILL.md for skills and traverses
links with cycle/directory limits. CLI denies mutation defaults; Claude exposes
Read/Glob/Grep and Commons MCP only, disables hooks and inherited settings; Codex
sets read-only sandbox, never approvals, and disables shell and multi-agent
features. Output parser rejects malformed, missing, and explicit failed results.
Supervisor cancels outstanding runs before waiting on shutdown/error.

## Pass 4 — verification

`go test -race ./internal/runtime` → PASS.
`go vet ./internal/runtime` → PASS.

Tests execute real child-process fixtures for Claude response parsing, Codex
resume and response parsing, cancellation, private MCP config lifecycle, and
Codex discovery handshake. Other tests cover invalid/failed completion, project
role content/provenance, linked skill and cycle traversal, and output bounds.
Root owns integrated core/supervisor response routing tests and actual model
round trips. Fixtures do not establish actual provider compatibility.

Known limits: CLI-version-specific adapters; no live interactive adoption; no
global configuration writes. Native runtime instruction discovery may additionally
load project context. Metadata discovery is not proof of managed reachability.
Markdown definitions only; other schemas are not parsed.

## Independent review follow-up

After independent review, Codex shell inspection is enabled under the explicit
read-only sandbox with approvals never and multi-agent disabled. Claude remains
Read/Glob/Grep plus Commons MCP. Cancellation now kills the owned process group;
a real fixture forks a descendant and verifies it exits. Failure evidence retains
bounded sanitized stderr and recognized JSON error fields from failed stdout,
including provider refusal labels without retrying or changing providers. A fixture
proves `[reasoning_extraction]` survives as diagnostic evidence. Credentials from
the session and common credential environment variables are redacted.

Every configured-state runtime run records a private mode0600 provenance receipt
under `receipts/`, containing session/delivery/runtime IDs, selected role source,
definition source hashes, exact rendered project prompt hash, and outcome. Receipt
contains no prompt content or credentials. Inventory cumulative content is capped
at 8 MiB. `go test -race ./internal/runtime` and `go vet ./internal/runtime` pass
after these changes. Parent owns real provider results; a provider refusal is not
treated as permission to retry through another route.
