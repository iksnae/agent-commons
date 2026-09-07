# Hermes: experimental visitor integration

Hermes work is deferred behind production Claude/Codex teams. It may later be the
operator-facing coordinator across workspaces; that is planned, not implemented.

Hermes 0.21.0's portable loader read this bundle's shared skill and MCP metadata
without diagnostics. This is parser evidence, not a working MCP connection.

A subsequent source inspection found that Hermes filters MCP child environments
and does not interpolate arbitrary environment variables in portable manifests.
The current portable manifest therefore does not pass the launcher's
`AGENT_COMMONS_CONNECTION` through. Do not enable it expecting automatic access.
The shared skill's explicit CLI route remains the documented participation path.

An isolated installer/MCP experiment is deferred. It is not part of the production
build, and no startup or automatic wake capability is claimed here.
