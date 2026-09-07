# Install a local binary bundle

Start with a native archive for your operating system and CPU from a build you
trust. There is no npm package or install-time download. Check `SHA256SUMS` for
accidental damage; a checksum shipped beside an archive does not prove who made it.
Signed public releases are still pending.

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
license and guide, source archive, and third-party notices. Symlinks are rejected.
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

`service-plan` renders launchd/systemd configuration for a selected binary and
state directory. Persistent service installation, role startup hooks and upgrade
automation remain open in [the beta checklist](BETA.md). Bundle installation does
not close those gates.
