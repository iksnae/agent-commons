// SPDX-License-Identifier: MPL-2.0

package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Unit tests exercise the settings-extraction helper against synthesised
// input, because a `go test` binary carries no vcs stamping of its own: the
// toolchain's -buildvcs=auto declines to stamp the synthetic test main
// package. That leaves the production path — ReadBuildInfo inside a real
// `go build ./cmd/agent-commons` — unproven by any unit test. This builds a
// genuinely stamped binary, serves with it, and reads the revision back off
// the wire, so a regression in that wiring cannot pass unnoticed.
func TestServedBuildIdentityMatchesStampedBinary(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	binary := filepath.Join(dir, "agent-commons")
	build := exec.Command("go", "build", "-o", binary, "./cmd/agent-commons")
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v: %s", err, output)
	}

	stamped, err := exec.Command("go", "version", "-m", binary).Output()
	if err != nil {
		t.Fatal(err)
	}
	revision := buildSetting(string(stamped), "vcs.revision")
	if revision == "" {
		// Archives are built with -buildvcs=false from a source snapshot with
		// no .git present, so there is no stamping to compare against.
		t.Skip("binary carries no vcs stamping; nothing to verify")
	}

	// macOS caps unix socket paths near 104 bytes, and a t.TempDir() path
	// named after this test overruns it, so the state directory is short.
	state, err := os.MkdirTemp("/tmp", "acstamp")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(state) })
	var serviceOutput strings.Builder
	service := exec.Command(binary, "serve", "--state", state)
	service.Stderr = &serviceOutput
	if err := service.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = service.Process.Kill()
		_, _ = service.Process.Wait()
	})
	token := filepath.Join(state, "operator.token")
	socket := filepath.Join(state, "service.sock")
	deadline := time.Now().Add(20 * time.Second)
	for {
		_, tokenErr := os.Stat(token)
		_, socketErr := os.Stat(socket)
		if tokenErr == nil && socketErr == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("service did not become reachable: %s", serviceOutput.String())
		}
		time.Sleep(50 * time.Millisecond)
	}

	reported, err := exec.Command(binary, "call", "--state", state, "--token-file", token, "runtime.status", "{}").Output()
	if err != nil {
		t.Fatal(err)
	}
	var status struct {
		Build map[string]any `json:"build"`
	}
	if err := json.Unmarshal(reported, &status); err != nil {
		t.Fatal(err)
	}
	if status.Build["vcsRevision"] != revision {
		t.Fatalf("served vcsRevision = %v, want the binary's own stamp %q", status.Build["vcsRevision"], revision)
	}
	if value, _ := status.Build["vcsTime"].(string); value == "" {
		t.Fatal("served build reported no vcsTime for a stamped binary")
	}
	// Presence, not truth: a clean tree reports false, and false must still
	// appear so it stays distinguishable from an unstamped build. This check
	// is weak here by nature — editing the source to break it dirties the
	// tree, which makes the value true and keeps the key present under
	// omitempty. The deterministic guard against a plain bool losing false
	// is the unit test; this asserts the wire carries the field at all.
	if _, ok := status.Build["vcsModified"]; !ok {
		t.Fatal("served build omitted vcsModified for a stamped binary")
	}
}

func buildSetting(output, key string) string {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "build" {
			if name, value, ok := strings.Cut(fields[1], "="); ok && name == key {
				return value
			}
		}
	}
	return ""
}
