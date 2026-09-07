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
    AGENT_COMMONS_NATIVE_SUPERVISOR=0 AGENT_COMMONS_NATIVE_SERVICE_CLI=0 go test -race ./...

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
check: fmt-check test vet

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
