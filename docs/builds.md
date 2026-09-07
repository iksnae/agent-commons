# Builds and archives

From the checkout root, `just build` produces `dist/dev/agent-commons` without
replacing the pilot binary. `just check` runs the race-enabled Go suite, vet,
format checks and Pi extension unit tests. Native agent and model tests are opt-in.

Stage reviewed new files before packaging: the build snapshots tracked working-tree
files, including uncommitted tracked changes, and excludes untracked files. Check
the diff first. Packaging is not a clean-tree or release-approval gate.

```sh
just package-check
```

This writes four native archives (macOS/Linux, AMD64/ARM64), a separate plugin
archive and `SHA256SUMS` to `dist/`. Each binary archive includes the guides and
plugin, plus source, vendored dependencies and required notices. Verification rebuilds all four binaries offline
from their source snapshots and compares bytes. It also exercises recoverable
host-native bundle installation in a temporary directory.

The GitHub Actions build workflow runs tests on macOS and Linux, then uploads
verified archives as an artifact named for the commit. It does not publish a
release, change repository visibility, register services or enroll target agents.
Checksums detect corruption; they do not authenticate the publisher.

Signed release artifacts and clean-machine native acceptance are still open in
[production readiness](../PRODUCTION.md). The [compatibility policy](version-compatibility.md)
documents the current pre-1.0 behavior. There is no Agent Commons
npm package. The plugin archive has its own installation and capability limits.
