# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

The operator's engineering rules for this repository live in AGENTS.md and are
authoritative. Read them; do not restate or override them here.

@AGENTS.md

## Session rooting

Module is `agentcommons`. Work must happen in this checkout, never in a target
repository. A shell `cd` does not root later tool calls — pass this checkout as
the working directory explicitly. `just resume` refuses the wrong checkout and
prints Git state (see START-HERE.md).

## Commands

`just` is the task surface; `Makefile` only has a reduced `check`/`build`.

- `just check` — pre-commit gate: `fmt-check`, `test`, `vet`, `pi-extension-test`,
  `hook-scripts-test`, `docs-check`, `install-check`.
- `just test` — `go test -race ./...` with every opt-in integration flag forced off.
- `just build` — dev binary to `dist/dev/agent-commons` (never replaces the running pilot's binary).
- `just fmt` / `just fmt-check` — gofmt over `cmd internal integration`.
- `just docs-check` — link and entry-point-length checks (`scripts/check-docs.mjs`).
- `just package-check` — build the four platform archives plus plugin, then verify them offline.
- `just plugin-check` — validate the Claude plugin manifest without installing.

Single test:

```sh
go test -race ./internal/core -run '^TestName$' -count=1
```

Opt-in suites are gated behind `AGENT_COMMONS_*` environment flags and each has a
`just` recipe: `native-test`, `claude-hook-test`, `codex-hook-test`, `pi-hook-test`,
`hermes-loader-test`, `skills-installer-test`, `live-test`. `just live-test` uses
real Claude/Codex models and consumes paid quota. Never set these flags inside
`just test` or CI's default path.

CI (`.github/workflows/ci.yml`) runs `go test -race ./...`, `go vet ./...`, the Pi
extension test and both docs checks on Ubuntu and macOS, then builds archives.

## Architecture

Four-layer split; dependencies point inward toward `internal/core`.

- `internal/core` — the domain and the only owner of persistence and state
  transitions. Standard library only. Exposes `New`, `Call(actor, method, params)`,
  `Authenticate`, `Token`, `Claim`, `Finish`, `Sessions`. Everything durable
  (registry, inbox, task/review ledger, versioned context, teams, boards,
  attachments) lives here with atomic writes and a single-process lock.
- `internal/transport` — local Unix-socket JSON-RPC plus the MCP stdio bridge.
  Authenticates a bearer token and delegates the method allowlist to core; it
  holds no coordination state. Socket 0600, state directory 0700.
- `internal/runtime` — the `Runner` interface and the Claude/Codex CLI adapter,
  plus `Serve` (a mechanical durable-queue poller, never an LLM asking to poll),
  discovery and project definition inventory. Discovery never adopts or rewrites
  a target.
- `internal/supervision`, `internal/installation`, `internal/codexlaunch`,
  `internal/codexrpc` — launchd/systemd process plans, bundle install/remove
  receipts, and Codex session preparation/resume/lease handling.
- `internal/console` — Bubble Tea read-only terminal UI. Charm dependencies are
  approved here and only here; UI state must not own coordination state.
- `cmd/agent-commons` — CLI orchestration only. `main.go` dispatches subcommands
  (`serve`, `call`, `mcp`, `init`, `enroll`, `check-in`, `doctor`, `console`,
  `service`, `bundle`, `wake-*`, `codex-*`, `discover`, `inventory`, `watch`,
  `methods`, `harnesses`). Keep orchestration out of the layers above.

Cross-cutting invariants enforced by tests: target isolation, idempotent sends,
immutable context versions, reviewer independence and no self-review,
`expectedRevision` CAS on submit/review, restart marking in-flight work
interrupted rather than replaying it, and credentials absent from every DTO.

`PLAN.md` is the interface contract — read it before changing any exported
signature or RPC method. `GATES.md` tracks acceptance gates with recorded
evidence; G5 (live cross-runtime) is open and intentionally not retried around.

## Documentation

`docs/README.md` indexes the guides and each guide owns its topic. Command
catalogs belong in `agent-commons help` and `methods` output, not duplicated into
docs. New Go source files carry `// SPDX-License-Identifier: MPL-2.0`.
