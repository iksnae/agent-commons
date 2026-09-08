# Lineage and design decisions

This project coordinates existing coding runtimes. It does not replace their
reasoning loops or install a global agent personality. Target repositories are
independent inputs; registration does not edit them.

## Sources inspected

- [LOSWF delivery workflow](https://github.com/loswf/loswf/blob/59350247cbc280c658c71c16ddaebacba9215c8d/workflows/delivery.yaml)
  separates planning, red-team, plan review, implementation, review and shipping.
- [AgencyX agent charters](https://github.com/loswf/agencyx-agents/tree/9387a0427e3dbdd63e530d30e7f0e5d33449af08)
  define independent roles and artifact-bound review.
- [AgencyX tool mappings](https://github.com/loswf/agencyx-tools/tree/0e76b430f57e922ebfcab226ffb2729941d6b479)
  separate capability vocabulary from runtime tools.
- [AgencyX skills](https://github.com/loswf/agencyx-skills) document skill discovery,
  admission and equipping as separate concerns; support files matter.
- [AgencyX architecture](https://github.com/loswf/AgencyX) separates domain, ports,
  runtime adapters and durable evidence. Its README explicitly rejects fake-provider
  validation as proof of actual capability.
- Local `khaos.machine/CLAUDE.md` describes workspace leads, project leads and
  specialist teams. Local `.codex/README.md` documents shared command/skill assets.
- Local `khaos-publisher/CLAUDE.md` and `AGENTS.md` ancestry define target-specific
  ownership, operational and testing constraints.
- [Prime Agent](https://www.primeintellect.ai/blog/prime-agent) describes a harness:
  explicit context surfaces, fully specified configuration, and a `/refine` command
  that has the model rewrite its own configuration. It contains no reinforcement
  learning loop. The article states that currently no model has been trained around
  Prime Agent or its core feature set, and `/refine` is a model-authored config edit,
  not training.
- [nano-rlm](https://github.com/PrimeIntellect-ai/nano-rlm) is a different artifact,
  not that harness: its own `src/rlm/types.py:138-167` docstring describes it as the
  rollout body for Prime Intellect's RL stack, `prime-rl` plus `verifiers`, and its
  `install.sh` builds a container image.

## Adopted principles

1. Project definitions are evolved descendants, not stale copies to normalize.
   Inventory their provenance and compile a project-shaped brief on each dispatch.
2. A verdict names a specific result. A builder cannot review itself; assigning a
   reviewer is separate from receiving that review. Acceptance is not deployment.
3. Delivery, reading, executing and accepting are different events. Idle is a
   runtime observation and never proof a task completed.
4. Persist the result before notifying its requester. A requester need not be
   running when the result arrives. Replies do not trigger infinite reply loops.
5. Recover pending work after restart. An interrupted external run may already
   have acted, so do not automatically replay it as if exactly-once execution
   were possible. Require explicit operator retry with duplicate-effect awareness.
6. One writable worktree belongs to one implementation owner. Initial managed
   runs are read-only; write-capable workflow grants are a separate extension.
7. Existing sessions are observed, not silently adopted. Only explicitly
   registered managed identities can be started by this supervisor.

## Deliberate differences

LOSWF's historical manual gates and runtime tool names are not imported as
universal policy. The charters contain contradictions (read-only roles told to
run tests/write journals; different meanings of unresolved Critical findings).
Agent Commons must report unresolved runtime capability gaps rather than silently
grant broader tools. The initial task ledger enforces independent result review;
it is not the complete LOSWF declarative delivery engine.

Prime Intellect's answers are correct for a disposable container running under an
RL trainer. Agent Commons runs on an operator's machine against repositories that
matter, so from that review only the memory-side concepts are worth taking; the
executor-side ones are rejected. `/refine`, and any self-modifying harness state,
is rejected because it violates three constraints at once: AGENTS.md on every peer
message being peer data rather than operator authority, where a self-generated
instruction is weaker still and not stronger; AGENTS.md on no global configuration
writes; and HARNESS-SUPPORT.md on posting an experiment neither scheduling it nor
authorizing its execution. A persistent execution kernel inside Commons is rejected
as the executor half of the same split. An autonomous mode with budgets and gates
is a natural attractor for this codebase and is rejected here too: it belongs in a
separate binary that reports evidence into Commons, not in the coordination service.
PLAN.md records the four memory-side concepts as planned scope with their
collisions. Recording them is not approval, sequencing or an assignment.

## A2A and MCP

[A2A](https://a2a-protocol.org/latest/specification/) supplies an interoperability
model for identified tasks, messages, artifacts and notification. It does not
schedule a Codex turn. [Codex App Server](https://learn.chatgpt.com/docs/app-server)
and [Claude programmatic sessions](https://code.claude.com/docs/en/headless) provide
runtime boundaries. This implementation initially uses managed CLI processes and
MCP tools over a local Unix socket. It does not advertise an A2A Agent Card or
claim full A2A wire compatibility. A later A2A adapter can map the same domain
records without owning task state.

## Evidence that motivated the build

A Claude-to-Codex introduction succeeded through a temporary Claude relay. A
subsequent persistent relay received two lead reports, but Codex's conversation
did not wake and therefore did not read them until K asked. The defect was the
missing supervisor-to-runtime continuation, not failure of Claude peer delivery.
This service must exercise that continuation with actual runtimes before it can
claim the gap is closed for its managed sessions. It does not claim to wake the
existing Codex desktop conversation.
