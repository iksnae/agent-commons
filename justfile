# SPDX-License-Identifier: MPL-2.0
set shell := ["bash", "-euo", "pipefail", "-c"]

# Show available development commands.
default:
    @just --list

# Compile a development binary without replacing the running pilot's binary.
build:
    mkdir -p dist/dev
    go build -o dist/dev/agent-commons ./cmd/agent-commons

# Run the credential-free suite with the race detector.
test:
    AGENT_COMMONS_HERMES_LOADER=0 AGENT_COMMONS_PI_HOOK=0 AGENT_COMMONS_SKILLS_INSTALLER=0 AGENT_COMMONS_NATIVE_SUPERVISOR=0 AGENT_COMMONS_NATIVE_SERVICE_CLI=0 AGENT_COMMONS_CLAUDE_HOOK=0 AGENT_COMMONS_CODEX_HOOK=0 go test -race ./...

# Run Go's static checks.
vet:
    go vet ./...

# Report unformatted Go files and fail if any are found.
fmt-check:
    @files="$(gofmt -l cmd internal integration)"; if [[ -n "$files" ]]; then echo "$files"; exit 1; fi

# Format project Go source and tests (changes files).
fmt:
    gofmt -w cmd internal integration

# Run ordinary pre-commit checks; no models or native service registration.
check: fmt-check test vet pi-extension-test

# Test Pi lifecycle behavior with fakes; no Pi installation or model calls.
pi-extension-test:
    node --test plugins/agent-commons/pi/commons.test.mjs

# OPT-IN: parse a copied bundle with Hermes's installed portable plugin loader.
# Set AGENT_COMMONS_HERMES_ROOT to an absolute Hermes checkout with venv/bin/python.
hermes-loader-test:
    AGENT_COMMONS_HERMES_LOADER=1 go test -race ./integration -run '^TestNativeHermesLoadsPortablePackage$' -count=1 -v -timeout 30s

# OPT-IN: use Pi's local package installer and metadata RPC in disposable config.
pi-hook-test: build
    AGENT_COMMONS_PI_HOOK=1 AGENT_COMMONS_TEST_BINARY="$PWD/dist/dev/agent-commons" go test -race ./cmd/agent-commons -run '^TestNativePiPackageChecksInExactSession$' -count=1 -v -timeout 90s

# Build all native archives and the plugin; stage new files first.
package:
    bash scripts/build-binaries.sh

# Verify existing archives, offline rebuilds, notices, and local bundle removal.
archives-check:
    bash scripts/check-archives.sh

# Build and verify distributable archives.
package-check: package archives-check

# Validate the Claude plugin manifest without installing it.
plugin-check:
    claude plugin validate plugins/agent-commons

# Show a role's read-only console; requires AGENT_COMMONS_CONNECTION.
console: build
    dist/dev/agent-commons console --config "${AGENT_COMMONS_CONNECTION:?Set a private role connection path}"

# Print scoped connection diagnostics; requires AGENT_COMMONS_CONNECTION.
doctor: build
    dist/dev/agent-commons doctor --config "${AGENT_COMMONS_CONNECTION:?Set a private role connection path}"

# OPT-IN: register a temporary native user job and test crash recovery. No models.
native-test: build
    AGENT_COMMONS_NATIVE_SUPERVISOR=1 AGENT_COMMONS_TEST_BINARY="$PWD/dist/dev/agent-commons" go test -race ./integration -run '^TestNativeSupervisorLifecycle$' -count=1 -v -timeout 8m

# OPT-IN: use actual Claude/Codex models; may consume paid quota. Retains evidence.
live-test:
    AGENT_COMMONS_LIVE=1 node integration/live.mjs

# OPT-IN: download pinned Vercel Skills CLI; install/remove in a disposable project.
skills-installer-test:
    AGENT_COMMONS_SKILLS_INSTALLER=1 go test -race ./integration -run '^TestVercelSkillsInstallerPreservesPortableSkill$' -count=1 -v -timeout 4m

# OPT-IN: run Claude init-only against isolated config/project; no model turn.
claude-hook-test: build
    AGENT_COMMONS_CLAUDE_HOOK=1 AGENT_COMMONS_TEST_BINARY="$PWD/dist/dev/agent-commons" go test -race ./cmd/agent-commons -run '^TestNativeClaudeLaunchHook$' -count=1 -v -timeout 45s

# OPT-IN: test Codex hook inventory, plugin lifecycle and thread identity in isolated config.
codex-hook-test:
    AGENT_COMMONS_CODEX_HOOK=1 go test -race ./integration -run '^TestNativeCodex(ReportsLaunchHookTrust|InstallsBundledPlugin|DistinguishesResumedAndForkedThreadIdentity|PreparesRecoverableRoot)$' -count=1 -v -timeout 90s
    AGENT_COMMONS_CODEX_HOOK=1 go test -race ./cmd/agent-commons -run '^TestNativeCodexPreparationCLI$' -count=1 -v -timeout 60s
