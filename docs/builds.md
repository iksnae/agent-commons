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

**`test-pass` is repo-wide and binary, so one failing test fails every Go unit.**
The certifier collects evidence once per module — a single `go test ./...` — and
copies that same result into every unit. The rule asks for a metric named
`test_failures`, which nothing emits (the test collector emits `test_failed`), so
evaluation falls through to a legacy branch that reads "a test run that did not
pass counts as 1". At `severity: error, threshold: 0`, one failure anywhere fails
all Go units at once. Only Go, because `test-pass` lives in `go.yml` and policy
packs are language-gated.

`TestCancellationKillsDescendants` in `internal/runtime` was the standing example
of how a flake reaches this amplifier by accident: it gave the fixture's child a fixed
150ms window to record its pid, so on a loaded machine it failed on a missing file
rather than on a surviving descendant. `9d5f778` replaced that window with a poll on
the pid file, and a reviewer stress-ran the pair 25 out of 25 clean under `-race`.
**A failure in that test is now new and reportable.** Do not match it to a known
flake and dismiss it.

This is not hypothetical. Measured against this configuration's 452 units, on one
machine and one commit, with `go vet` clean in both runs — the failing case produced
by injecting a deliberately failing test:

| Test run | Result | Report card |
| --- | --- | --- |
| 0 failures | **452 of 452 units certified** | — |
| 1 failure | **20 of 452 units certified** | `overall_grade: B`, `pass_rate: 0.044` |

Nothing about the graded code changed between those runs. Two things deserve
attention in the second row. The first is the floor: a single failing test moves
the result from everything passing to 4.4% passing. The 20 survivors are exactly
the units no rule examines — 14 shell, 5 JavaScript, 1 Python — because
`test-pass` reaches only Go; that is what the pass rate is measuring.

The second is that **the grade stays B anyway**, and it is worth being precise
about why, because the intuitive explanation is wrong. `overall_grade` is the
arithmetic mean of every unit's score, taken with no reference to whether the
unit passed. Grade and certification status are therefore decoupled: failing the
binary `test-pass` rule costs a unit only a little score, so all 432 Go units can
fail certification while 380 of them still grade B. The unchecked survivors are
not propping the letter up — they are 4.4% of the weight and average 0.806
against Go's 0.838, so dropping them entirely would nudge the mean *up*, from
0.836 to 0.838, and leave the grade at B. The B is produced by the 432 failing
units themselves.

A red certification badge is therefore evidence about one test's timing at least
as often as it is evidence about the codebase, and the same collapse can be
produced by a broken module environment, since `go: downloading …` chatter is also
recorded as failure. Check which test failed, and check the pass rate rather than
the letter grade, before reading anything into the result. This is the single
largest reason to treat certification as advisory — which is why `config.yml` sets
`mode: advisory`.

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
was removed, so the 20 non-Go units in the index — 14 shell, 5 JavaScript, 1
Python, against 432 Go — currently match no rules and score the default. Their
grades say nothing was checked, not that everything passed.

This is deliberate. Go is what certification is here to measure; the shell and
JavaScript sources have their own gates in `just check`, which certification does
not replace. The cost is the collapse behaviour above: because no rule reaches
them, these 20 are the only units that can still pass once a Go test fails, so
the pass rate is reporting unchecked code. They do not distort the letter grade —
they score below the Go mean, so removing them would raise it — but they are the
reason a 4.4% pass rate is not a 0% one.

`unsafe_import_count` is likewise no longer enforced; see the
note in `.certification/policies/go.yml` for why counting that metric could not
express what this project needs.

**AI-assisted review is disabled, and the disable is narrower than it looks.**
`.certification/config.yml` sets `agent.enabled: false` deliberately, for
security rather than preference: left unset, `certify certify` detects provider
credentials in the ambient environment and uploads unit source to whichever
provider it finds. The disable stops that.

It does not stop `certify scan`, which runs first in all three workflows. Scan
reads only the `scope` block of the config, then detects providers and calls out
regardless, and it has no `--skip-agent` flag to suppress. What it sends is a
repository summary — language names, unit count, adapter names — never unit
source, and any failure is swallowed silently. GitHub Actions runners have no
provider keys and no local model server, so CI is unaffected; the exposure is
running `certify scan` on a workstation that holds keys. Read the comment in
`config.yml` before changing any of this.
