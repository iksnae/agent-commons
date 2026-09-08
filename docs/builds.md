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

## Certification: what the grade does and does not mean

Three workflows in `.github/workflows/` run [certify](https://github.com/iksnae/code-certification)
over this repository and post a report card. Read the number with these limits in
hand, because each one was measured here rather than assumed.

**Every run is a cold certification.** certify keeps incremental state in
`.certification/state.json` and `records/*.history.jsonl`. Both are generated
output and neither is tracked, so a fresh CI checkout begins with no history.
`config.yml` sets `expiry.default_window_days: 90`, but a window that is never
carried between runs can never elapse. Every push therefore recertifies every
unit from scratch instead of the units the push touched. The result is correct
and costs more than the configuration suggests. Keeping the history would mean
caching it across runs, which is a separate decision from tracking the inputs.

**`test-pass` is repo-wide and binary, so one flaky test fails everything.** The
certifier collects evidence once per module — a single `go test ./...` — and
copies that same result into every unit. The rule asks for a metric named
`test_failures`, which nothing emits (the test collector emits `test_failed`), so
evaluation falls through to a legacy branch that reads "a test run that did not
pass counts as 1". At `severity: error, threshold: 0`, one failure anywhere
fails all units at once.

This is not hypothetical. `TestCancellationKillsDescendants` in
`internal/runtime` is a known load-sensitive flake: it passes when run alone and
fails intermittently under a full `go test ./...`. Measured on the same commit
and the same machine, with a healthy toolchain and `go vet` clean both times:

| Run | Flake | Result |
| --- | --- | --- |
| Full suite, flake tripped | 1 of 552 failed | **0 of 597 units certified** |
| Full suite, flake passed | 0 of 552 failed | **597 of 597 units certified** |

Nothing about the code changed between those two runs. A red certification badge
is therefore evidence about one test's timing at least as often as it is evidence
about the codebase, and the same collapse can be produced by a broken module
environment, since `go: downloading …` chatter is also recorded as failure. Check
which test failed before reading anything into the grade. This is the single
largest reason to treat the certified count as advisory — which is why
`config.yml` sets `mode: advisory`.

**`lint-clean` counts output lines, not diagnostics.** The `go vet` parser treats
every non-blank line that does not begin with `#` as one lint error. Module
download notices and other toolchain chatter are counted as findings, so on a
cold module cache the rule measures how talkative the toolchain was rather than
what vet actually found.

**Scope excludes files, not Go symbols.** Scope patterns are matched against a
file's basename and are passed only to the generic file scanner, never to the Go
symbol adapter. A pattern like `**/*_test.go` matches nothing at all and must be
written `*_test.go`. Excluding test files removed 145 file units here (597 → 452)
and no symbols, because the Go adapter emits no symbols from `_test.go` files —
verified by inspecting the index. On a codebase where it did, the exclusion would
not reach them.

**Some units are graded against no rules.** Policy packs are language-gated: a
pack with no `language` key applies to every unit, one with `language: go` only
to Go. `lint-clean` and `test-pass` now live only in `go.yml`, and `python.yml`
was removed, so shell, TypeScript, Markdown, YAML and Python units currently match
no rules and score the default. Their grades say nothing was checked, not that
everything passed. `unsafe_import_count` is likewise no longer enforced; see the
note in `.certification/policies/go.yml` for why counting that metric could not
express what this project needs.

AI-assisted review is disabled in `.certification/config.yml`, deliberately and
for security rather than preference. Left unset, certify detects provider
credentials in the ambient environment and uploads unit source to whichever
provider it finds. Read the comment there before changing it.
