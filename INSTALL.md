# Install a local binary bundle

Start with the shared harness instructions in the target project. The binary
bundle has no npm package or install-time download. Check `SHA256SUMS` for
accidental damage; a checksum shipped beside an archive does not prove who made it.
Signed public releases are still pending.

## Install the CLI with the install script

`scripts/install.sh` is the ordinary developer-CLI route. You run it; running it
is the consent. It installs one binary and does nothing else.

```sh
curl -fsSL https://raw.githubusercontent.com/iksnae/agent-commons/main/scripts/install.sh | bash
```

It detects your platform, downloads the matching archive and `SHA256SUMS` from the
latest GitHub release, refuses to install on a checksum mismatch, and copies the
binary to `~/.local/bin`. It never uses `sudo`, never edits a shell profile, never
writes global configuration, and never starts a service or enrolls a role. If the
install directory is not on your `PATH` it prints the line to add and leaves the
file to you.

Options need `bash -s --`, because the piped shell reads the script on standard
input and would otherwise take the flags as its own:

```sh
curl -fsSL https://raw.githubusercontent.com/iksnae/agent-commons/main/scripts/install.sh \
  | bash -s -- --to /absolute/your/bin
```

`--archive PATH` installs an archive you already have and skips the download. That
is the route to use before any release is published. When a `SHA256SUMS` sits beside
the archive the script checks against it; when none does, it says so and installs the
file you named rather than pretending to a guarantee it does not have.

The checksum detects transfer damage, not publisher authenticity. Release signing
is still pending, so treat a verified checksum as an intact download and no more.

The script asks GitHub for the *latest* release, which by definition excludes drafts
and prereleases. A first tag cut as a prerelease returns nothing, and the script
reports that no published release was found while the release is plainly visible on
the repository page. Publish the first release as a normal, non-draft release, or
install from a local archive with `--archive`.

## Install the shared skill with Vercel Skills

Use the established [Vercel Skills CLI](https://github.com/vercel-labs/skills) for
the instructions and their supporting references:

```sh
DISABLE_TELEMETRY=1 npx skills@1.5.24 add \
  https://github.com/iksnae/agent-commons.git \
  --skill agent-commons --agent claude-code codex pi hermes-agent --copy
```

Select only the agents you use. This installs at project scope; add `--global`
only when you want a user-wide skill. `--copy` keeps support files together
without depending on symlinks.

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
Because nothing is added to PATH, the installed copies of the plugin's MCP
manifests name the binary this install placed instead of a bare command. The
bundle you unpacked is not modified.

Keep service state and connection files outside the installation directory.
Do not store notes or credentials in it. A failed copy is retained for inspection
and reported as an incomplete installation, never as success.

## Check or remove an installation

```sh
/absolute/installed/agent-commons bundle verify --to /absolute/installed
```

This compares files and permissions against a local receipt, and checks that each
MCP manifest still names a binary that exists here and is executable. It detects
changes, not forged receipts or an untrusted publisher. Moving an installation
after installing it strands that command, so verify reports it as unusable.

Stop every service and watcher using this binary before removing it:

```sh
/absolute/installed/agent-commons bundle remove \
  --to /absolute/installed --confirm-stopped
```

Removal refuses changed files or added files. It moves the verified directory
into a private sibling archive and prints the recovery path. Nothing is deleted.
Move that retained directory back to its original, now-vacant path to restore it.
Verify reports the retained copy as unusable until you move it back, because its
MCP command still names the original path.
The command does not inspect or stop the OS supervisor. `--confirm-stopped` is your
confirmation, not a claim that the tool checked your processes.

## Update the CLI in place

`agent-commons version` prints the release the binary was built from. Release
archives are built without VCS stamping, so a binary that reports `dev` was built
from a working tree rather than cut from a tag.

```sh
agent-commons update
```

This asks GitHub for the latest published release of `iksnae/agent-commons`. When
that tag is the one this binary reports, it says so and downloads nothing. Otherwise
it downloads the archive for your platform along with `SHA256SUMS`, checks the archive
against them, and only then writes the new binary beside the running one and renames
it over the top. A mismatch replaces nothing.

It does not compare version numbers. GitHub decides which release is latest; this
command reports what that release is and installs it, rather than claiming a release
is newer than yours. A binary reporting `dev` matches no tag, so running this command
on a locally built binary says the build carries no release stamp and then overwrites
it with the published release. Keep a build you still need somewhere else first.

The checksum detects transfer damage, not publisher authenticity. Release signing is
still pending, so treat a verified checksum as an intact download and no more.

The command replaces the binary it is running from. If that file's directory is not
writable it names the path and stops; it never uses `sudo` and never elevates. Install
somewhere you own with `scripts/install.sh --to DIR` instead. A running service keeps
the old binary until it is restarted, and this command does not restart it.

This route updates one binary. It does not update the bundle, the plugin, or the
installed skill.

`update` first shipped in `v0.0.1`, so a binary installed before that tag does not
have the command and cannot use it to reach `v0.0.1`. Run `scripts/install.sh` for
that first step. The script replaces the installed file the same way this command
does — it stages the new binary inside the install directory and renames it over
the old one — so a process already running from that path keeps the file it started
from rather than having its image overwritten underneath it. As with `update`, a
running service keeps the old binary until you restart it.

## Upgrade and service setup

Install a new version into a separate directory. Verify it, back up service state,
then schedule a stop and switch to the new binary. Retain the old installation and
the matching state backup until the migration and rollback have been tested.
Do not run old and new services against one state directory at the same time.

The [compatibility policy](docs/version-compatibility.md) describes which state
and connection versions a binary may accept. It does not make native provider
sessions portable across releases.

`service-plan` renders launchd/systemd configuration without writing it.
The [service commands](SERVICE.md) install and control that configuration explicitly.
Role startup hooks, login/reboot validation and upgrade automation remain open
in [the beta checklist](BETA.md). Bundle installation does not close those gates.
