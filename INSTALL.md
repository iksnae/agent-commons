# Install a local binary bundle

Start with a native archive for your operating system and CPU from a build you
trust. The binary bundle has no npm package or install-time download. Check `SHA256SUMS` for
accidental damage; a checksum shipped beside an archive does not prove who made it.
Signed public releases are still pending.

## Install the shared skill with Vercel Skills

Use the established [Vercel Skills CLI](https://github.com/vercel-labs/skills) for
the instructions and their supporting references. From the target project, with
a trusted local Agent Commons checkout available:

```sh
DISABLE_TELEMETRY=1 npx skills@1.5.24 add /absolute/agent-commons/plugins/agent-commons \
  --skill agent-commons --agent claude-code codex pi hermes-agent --copy
```

Select only the agents you use. This installs at project scope; add `--global`
only when you want a user-wide skill. `--copy` keeps support files together
without depending on symlinks. Vercel also supports Git sources; the repository
is currently private, so remote installation needs your existing Git access.

The command downloads Vercel's npm-published installer, not an Agent Commons npm
package. It does not install our binary, register MCP, start a service, enroll a
role or grant work authority. Keep the operator-provided connection separate.
Use `npx skills@1.5.24 remove agent-commons` to remove a skills-only installation.

Claude/Codex native plugin installation is a separate route when you need the
bundled MCP configuration and supported startup hook. Do not install duplicate
skill copies through both routes for the same role. The same extracted plugin
directory also installs through `pi install /absolute/path/to/agent-commons`;
see its README for explicit launch binding and limits. Hermes can read the
bundle's portable Agent Plugins manifests; the plugin README describes its
experimental status and the remaining native acceptance checks. Hermes MCP
environment binding is not ready; do not treat parsed metadata as a working connection.

`just skills-installer-test` exercises the real pinned installer in disposable
directories. Ordinary tests do not download it or invoke native agents.

## Install the binary bundle

Unpack the archive in a scratch directory. For example, the macOS ARM64 archive
contains an `agent-commons-darwin-arm64` directory. Run its binary to copy the
complete bundle into a new, dedicated installation directory:

```sh
/absolute/unpacked/agent-commons bundle install \
  --from /absolute/unpacked \
  --to /absolute/installations/agent-commons-v0.1.0
```

The parent installation directory must already exist. The destination itself must
not exist, even as an empty directory. The command copies the binary, project
license and guides, integration plugin, source archive, and third-party notices.
Plugin executable permissions are retained, but its scripts are not run during
installation. Symlinks are rejected.
Nothing is added to PATH, no service is started, and no agents are enrolled.

Keep service state and connection files outside the installation directory.
Do not store notes or credentials in it. A failed copy is retained for inspection
and reported as an incomplete installation, never as success.

## Check or remove an installation

```sh
/absolute/installed/agent-commons bundle verify --to /absolute/installed
```

This compares files and permissions against a local receipt. It detects changes,
not forged receipts or an untrusted publisher.

Stop every service and watcher using this binary before removing it:

```sh
/absolute/installed/agent-commons bundle remove \
  --to /absolute/installed --confirm-stopped
```

Removal refuses changed files or added files. It moves the verified directory
into a private sibling archive and prints the recovery path. Nothing is deleted.
Move that retained directory back to its original, now-vacant path to restore it.
The command does not inspect or stop the OS supervisor. `--confirm-stopped` is your
confirmation, not a claim that the tool checked your processes.

## Upgrade and service setup

Install a new version into a separate directory. Verify it, back up service state,
then schedule a stop and switch to the new binary. Retain the old installation and
the matching state backup until the migration and rollback have been tested.
Do not run old and new services against one state directory at the same time.

`service-plan` renders launchd/systemd configuration without writing it.
The [service commands](SERVICE.md) install and control that configuration explicitly.
Role startup hooks, login/reboot validation and upgrade automation remain open
in [the beta checklist](BETA.md). Bundle installation does not close those gates.
