# Resume Agent Commons development

Open your agent session in the Agent Commons checkout, not in a target project.
This repository's Go module is `agentcommons`. A shell `cd` does not change the
default directory of later tool calls; supply the absolute checkout as `workdir`
on every repository command.

From this checkout, run `just resume`. From another directory, pass the absolute
path to this checkout's justfile with `just --justfile PATH/justfile resume`.
Just runs the recipe from its justfile directory. This checks the root and prints
Git state without reading credentials, enrolling roles or starting agents.

## Read only what you need

1. [Engineering rules](AGENTS.md).
2. [Production priorities and open gates](PRODUCTION.md).
3. [Implementation contract](PLAN.md) for interfaces being changed.
4. [Documentation index](docs/README.md) for the current task.

## Current handoff

Claude and Codex working teams come first: launch/resume with scoped credentials,
durable delegation and return paths, then team-scoped work and independent review.
The foundation is still a read-only pilot. Green builds do not close the live
cross-runtime gate or authorize unattended repository writes.

Hermes is deferred. It may later be the operator's point of contact for organizing
teams across workspaces, including another host. Cross-host operation and shared
system access are not implemented. They need explicit scope and revocable grants.

The previous Hermes experiment was preserved in a local Git stash named
`Deferred Hermes native installer and MCP experiment`. Stashes are local, not
shipped or pushed. Inspect the stash list before attempting recovery; do not apply
it automatically. Its native test failed before MCP execution because the plugin
data directory was not private enough. Its portable connection changes are WIP.

Keep the running pilot, its credentials and existing native agent sessions alone.
Use disposable service state for tests. Never retry around the recorded provider
safeguard refusal to make an acceptance gate pass.

## Before handing off

Run `just check`, then build and verify archives when packaging changes. Obtain
independent review. Record exact evidence and remaining gates, commit only reviewed
work, and leave the next session rooted in this repository. Public release,
signing, live upgrades and target-agent enrollment remain separate decisions.
