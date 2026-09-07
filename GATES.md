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
  EVIDENCE: LIVE-EVIDENCE.md records one completed real round trip and a successful reverse request after restart; final resumed Claude acknowledgement refused by provider [reasoning_extraction]. Full bidirectional acceptance remains unproven.
  ABANDON: G5 This live attempt is blocked by an external provider safeguard refusal; no retry/model substitution to work around it. Other automated and implementation gates remain independently verified.
- [x] G6: Independent adversarial review resolved, source lineage and limitations documented.
  EVIDENCE: REVIEW.md records independent core/transport and runtime findings, fixes and reviewer rechecks; RESEARCH.md pins LOSWF source lineage; README.md and LIVE-EVIDENCE.md disclose limitations.
