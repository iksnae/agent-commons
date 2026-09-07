# Agent Commons

Agent Commons is a local service, CLI and integration plugin for project-shaped
Claude/Codex teams, with durable inboxes, shared context and independent review.

Status: read-only local pilot, not production-ready. The
[release gates](PRODUCTION.md) remain open. Existing sessions are never adopted
automatically, and installing the plugin does not grant work authority.

## Start here

[Install the binary and shared skill](INSTALL.md), then
[enroll a project role](integrations/ONBOARDING.md). Use the separate
[plugin guide](plugins/agent-commons/README.md) for native integration.
Keep credentials and service state outside target repositories.

Building from this checkout requires Go 1.26, Just, and Node.js for plugin tests.
From the repository root:

```sh
just build
dist/dev/agent-commons help
dist/dev/agent-commons harnesses
```

The binary is built at `dist/dev/agent-commons`; this does not replace a running
service. Actual model work also requires an installed, authenticated runtime.
For background service setup, use the [service guide](SERVICE.md).

## Find the right guide

- [Documentation index](docs/README.md): coordination, notifications and operations.
- [Runtime support](HARNESS-SUPPORT.md): implemented capabilities and their limits.
- [Production readiness](PRODUCTION.md): evidence required before release.
- [Resume development](START-HERE.md): repository rooting and current priorities.

Project definitions remain owned by their targets. Commons records coordination;
a delivered message is not proof of reading, and task acceptance is not deployment.
System-wide sharing and cross-workspace orchestration remain planned.

## Build and verify

```sh
just check
```

[Build and archive verification](docs/builds.md) covers local artifacts and CI.
There is no Agent Commons npm package. Vercel Skills is a separate installer for
the shared instructions.

## License

[MPL-2.0](LICENSE). See the [plain-language guide](LICENSING.md) for distribution
obligations. The software comes without warranty or a promise of ongoing support.
