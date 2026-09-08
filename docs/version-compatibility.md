# Version and compatibility policy

Agent Commons is still pre-1.0. A green build is not a promise that every
upgrade can reuse an existing native session or state directory.

## Durable service state

The `state.json` schema is monotonic. A binary accepts its own schema and older
schemas it knows how to migrate. It refuses newer schemas. The team schemas
(1 to 2 and 2 to 3), the task-abandonment schema (to 4) and the
session-retirement schema (to 5) each take a private pre-migration snapshot
before the first write; keep that snapshot until the upgrade has been validated.
A schema number is never shared between two features. Do not try to downgrade by
changing a schema number by hand. That is an operator rule, not a claim that
every hand-edited file can be detected.

Schema 4 is taken lazily, the first time `tasks.abandon` actually succeeds. A
refused abandonment writes no snapshot and leaves the number alone, and a state
directory that never abandons a task keeps its existing schema and stays
readable by an older binary. Once a task is abandoned the directory is
deliberately no longer downgradable: an older binary decides task terminality by
testing `accepted` alone, so it would let an abandoned task be resubmitted and
accepted, which is the one outcome abandonment exists to prevent. It refuses an
unknown schema on startup, so raising the number is what turns that refusal on.

Schema 5 is taken the same way, the first time `sessions.retire` succeeds. A
refused retirement writes no snapshot and leaves the number alone. Once an
identity is retired the directory is no longer downgradable: an older binary has
no notion of a retired identity, so it would list a withdrawn role as active,
let `tasks.assign` name it and let `sessions.enroll` adopt it. Loading also
refuses a schema-5 state in which a retired identity still holds a credential,
because credential destruction is meant to be durable state and not merely
something the code path did once.

Schema-0 startup performs the built-in legacy normalization and stranded-run
recovery before saving. That is distinct from the team migrations and is not a
full backup. A failed migration must leave the original state usable.
Restore and reconciliation procedures are not yet production-complete; do not
treat a schema snapshot as a full backup.

## Role connections and transports

Connection files currently use version `1`. They are private, project-specific
bindings for one project plus agent name and role. Unknown fields are rejected;
do not copy a connection file between projects or roles. RPC and MCP clients
must negotiate capabilities and use the advertised method contract. A missing
capability means that feature is unavailable, not that it should be guessed.

## Command output (breaking change in v0.0.3)

`init`, `enroll`, `doctor` and `bundle` now print a human summary by default.
They printed a JSON document before. Pass `--json` to any of those four to get
that document back; it is unchanged, field for field. If a script reads their
stdout, add `--json` to the invocation before upgrading past v0.0.2.

`check-in` did not change and has no `--json` flag. Its stdout is still exactly
one JSON document and nothing else, because harnesses parse it whole and
`--hold` streams arrivals onto the same stream. It now writes a short human
header to stderr, which callers that only read stdout never see.

`update` gained no flag. It printed prose before and prints prose now. Every
other command — `call`, `watch`, `methods`, `connect-mcp`, `harnesses`,
`discover`, `inventory`, `launch-context`, `service`, `wake-*` and `codex-*` —
is unchanged.

Colour appears only on a terminal. A pipe, a redirect and `NO_COLOR` all get
plain text. Label columns are padded either way.

## Plugins and native adapters

The Claude, Codex, Pi and Hermes integration bundles are versioned with the
release that produced them. They are adapters, not replacements for native
transcripts or credential stores. A native session ID is an attachment and may
not be reused after a fork or scope mismatch. Native launch, upgrade, login and
reboot acceptance is still an open production gate.

Until a signed release policy is published, pin the exact source commit and
archive checksums used for an installation. Do not auto-upgrade a live service,
plugin, role connection or target project.
