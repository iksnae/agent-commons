# Managed process environment

Managed Claude and Codex runs no longer inherit every variable from the Commons
service. They receive an explicit allowlist. Unknown variables are omitted,
including unrelated credentials, the Commons operator token, shell startup files,
dynamic-library injection settings and `NODE_OPTIONS`.

Both harnesses retain `PATH`, `HOME`, `TMPDIR`, `LANG`, `LC_ALL`, `LC_CTYPE` and `TZ`.
They also retain HTTP/HTTPS/ALL/NO proxy variables in upper and lower case,
`SSL_CERT_FILE`, `SSL_CERT_DIR` and `NODE_EXTRA_CA_CERTS`. These are operator-owned
network and certificate settings, not settings supplied by peer messages.

## Authentication

Codex retains `OPENAI_API_KEY`, `CODEX_API_KEY`, `CODEX_ACCESS_TOKEN`, `CODEX_HOME`,
`CODEX_SQLITE_HOME` and `CODEX_CA_CERTIFICATE`. Claude retains
`ANTHROPIC_API_KEY`, `CLAUDE_CODE_OAUTH_TOKEN` and `CLAUDE_CONFIG_DIR`. A credential
for one harness is not forwarded to the other. See the native
[Codex environment](https://learn.chatgpt.com/docs/config-file/environment-variables) and
[Claude environment](https://code.claude.com/docs/en/env-vars) references for their
meaning. Commons does not copy credentials, log them or perform login.

Managed provider profiles are not implemented yet. Nonempty `OPENAI_*` variables
outside the allowlist stop a Codex run before launch. Claude similarly rejects
unlisted `ANTHROPIC_*`, `CLAUDE_CODE_USE_*` and `CLAUDE_CODE_OAUTH_*` variables.
This includes custom endpoints, federation credentials, cloud-provider selectors
and Claude model overrides. Rejecting these settings avoids silently falling back
to another provider, account or model. Errors name the setting, never its value. Configure a
separate direct-provider service environment only if that is your intended route;
do not unset a required gateway merely to get past this check.

## Limits

This is environment filtering, not full role isolation. The process still has
its configured home, native credential store and normal OS access. Two roles
using the same provider can still use the same provider credential. Scoped
Commons credentials are supplied separately through a private per-run token file.

Subprocess fixtures verify the child environment without calling a model. Native
provider authentication, role-specific credential provisioning and the isolated
Codex preparation path still need acceptance. Discovery and standalone native
preparation are separate paths; this policy covers managed model execution only.
