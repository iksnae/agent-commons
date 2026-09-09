# Gates: Agent Commons

Scope: Independent local Claude/Codex team coordination with durable responses.

- [x] G1: Durable scoped registry, messaging, context and independent review gates.
  CHECK: go test ./internal/core
  EXPECT: ok
  EVIDENCE: ok  	agentcommons/internal/core	1.036s
- [x] G2: Managed runtimes serialize work and deliver replies without a human relay.
  CHECK: go test ./internal/runtime
  EXPECT: ok
  EVIDENCE: ok  	agentcommons/internal/runtime	0.074s
- [x] G3: Authenticated local CLI and MCP share a consistent tool contract.
  CHECK: go test ./internal/transport ./cmd/agent-commons
  EXPECT: ok
  EVIDENCE: ok  	agentcommons/internal/transport	0.088s | ok  	agentcommons/cmd/agent-commons	0.044s
- [x] G4: Integrated tests, race detector, vet and build pass.
  CHECK: go test -race ./... && go vet ./... && go build -o bin/agent-commons ./cmd/agent-commons
  EXPECT: ok
  EVIDENCE: ok  	agentcommons/internal/runtime	(cached) | ok  	agentcommons/internal/transport	(cached)
- [ ] G5: Actual Claude and Codex runs prove cross-runtime delivery and continuation.
  EVIDENCE: `AGENT_COMMONS_LIVE=1 node integration/live.mjs`, 2026-09-07, default configured models, scratch target. A registered Codex identity submitted to a managed Claude identity and a duplicate submission returned the same delivery ID; Claude read the shared context through `mcp__commons__context_get`; the service persisted the result and automatically started Codex, which acknowledged without a new human message (delivery 1658d95a0d2bcdd9d1b537d512ce5ae7f142bf2548e3c452, result bc6268af1a7b3ef3e011734ebb8f7958f185b42491061772, both `completed`); after a stop/restart on existing state the reverse request reached the existing managed Codex identity and completed. The final resumed Claude acknowledgement failed with `invalid_request` and a provider safeguard flag [reasoning_extraction], request ID req_011CepKoKPzeTHM3TzazoMk9; no model substitution or rewritten request was used to work around it. Full bidirectional acceptance remains unproven. This run's local write-up is not in the repository — `.gitignore` excludes it because its state directory holds credentials — so the summary above is the citable record.
  ABANDON: G5 This live attempt is blocked by an external provider safeguard refusal; no retry/model substitution to work around it. Other automated and implementation gates remain independently verified.
- [x] G6: Independent adversarial review resolved, source lineage and limitations documented.
  EVIDENCE: REVIEW.md records independent core/transport and runtime findings, fixes and reviewer rechecks; RESEARCH.md pins LOSWF source lineage; README.md and G5 above disclose limitations.
