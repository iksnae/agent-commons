# Version and compatibility policy

Agent Commons is still pre-1.0. A green build is not a promise that every
upgrade can reuse an existing native session or state directory.

## Durable service state

The `state.json` schema is monotonic. A binary accepts its own schema and older
schemas it knows how to migrate. It refuses newer schemas. The team schemas
(1 to 2 and 2 to 3) take a private pre-migration snapshot before the first
write; keep that snapshot until the upgrade has been validated. Do not try to
downgrade by changing a schema number by hand. That is an operator rule, not a
claim that every hand-edited file can be detected.

Schema-0 startup performs the built-in legacy normalization and abandoned-run
recovery before saving. That is distinct from the team migrations and is not a
full backup. A failed team migration must leave the original state usable.
Restore and reconciliation procedures are not yet production-complete; do not
treat a schema snapshot as a full backup.

## Role connections and transports

Connection files currently use version `1`. They are private, project-specific
bindings for one project plus agent name and role. Unknown fields are rejected;
do not copy a connection file between projects or roles. RPC and MCP clients
must negotiate capabilities and use the advertised method contract. A missing
capability means that feature is unavailable, not that it should be guessed.

## Plugins and native adapters

The Claude, Codex, Pi and Hermes integration bundles are versioned with the
release that produced them. They are adapters, not replacements for native
transcripts or credential stores. A native session ID is an attachment and may
not be reused after a fork or scope mismatch. Native launch, upgrade, login and
reboot acceptance is still an open production gate.

Until a signed release policy is published, pin the exact source commit and
archive checksums used for an installation. Do not auto-upgrade a live service,
plugin, role connection or target project.
