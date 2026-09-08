# Transport and CLI leaf evidence

Owned paths: `internal/transport/**`, `cmd/agent-commons/**`, this file.

Four implementation passes:

1. Contract: read PLAN.md and GATES.md completely; matched core/runtime interfaces and established common dotted RPC/MCP tool names. Administrative registration and retry remain outside MCP tool discovery.
2. Build: authenticated Unix HTTP RPC, newline JSON-RPC MCP, explicit session credential file, operator CLI, service/runtime lifecycle, target inventory and Claude/Codex discovery commands.
3. Adversarial self-check: covered missing/invalid credentials, forged sender payload, agent registration denial, unknown methods, trailing request objects, MCP administrative tool denial, clean protocol output, credential permissions and symlink rejection. Identity derives from authenticated core actor; a supplied sender cannot impersonate operator. This is builder verification, not independent approval.
4. Verification: package race tests and vet passed; CLI lifecycle test exercises real Unix socket startup, operator credential materialization, RPC query, bad input, and cancellation. No live agent contacted and no global configuration installed by this leaf.

Commands:

```
go test -race ./internal/transport ./cmd/agent-commons
go vet ./internal/transport ./cmd/agent-commons
```

Limits: MCP supports newline JSON-RPC over stdio, not HTTP MCP or A2A. Existing socket paths are not replaced automatically; an operator must resolve stale endpoints. Session tool calls enforce scope in core; possession of operator credentials intentionally grants operator authority. Socket/state credentials protect against other OS users, not processes already running as the same OS user. Discovery is read-only and does not adopt sessions. Flags precede positional METHOD/JSON arguments. `init` enrollment is not transactional with the manifest write: once `enrollAgent` returns, a session has been minted or adopted and a connection file exists, and any later failure strands it with no manifest and no report naming it — a target that is readable and traversable but not writable reaches that state through `MkdirAll`; the two ways to close it are recorded at the call site in `cmd/agent-commons/init.go`. The adopted-versus-created label these commands report degrades to `unknown` when the pre-RPC `sessions.list` snapshot fails, and that downgrade is untested because the snapshot and the enroll RPC share one socket and one operator token; a stdlib-only route to a test, needing no production injection point, is recorded at `enrollAgent` in `cmd/agent-commons/enrollment.go`.
