// SPDX-License-Identifier: MPL-2.0

package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// A consumer once read an absent method as unshipped when the running process
// simply predated it, because no response said which build was answering.
func TestRuntimeStatusReportsBuildIdentity(t *testing.T) {
	s, _, _ := setup(t)
	page := rpc(t, s, "operator", "runtime.status", map[string]any{}).(RuntimeStatusPage)
	started, err := time.Parse(time.RFC3339Nano, page.Build.StartedAt)
	if err != nil {
		t.Fatalf("startedAt not RFC3339Nano: %q: %v", page.Build.StartedAt, err)
	}
	if started.IsZero() || time.Since(started) > time.Minute {
		t.Fatalf("startedAt not this process: %s", page.Build.StartedAt)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Skip("executable path unavailable")
	}
	data, err := os.ReadFile(executable)
	if err != nil {
		t.Skip("executable unreadable")
	}
	sum := sha256.Sum256(data)
	if page.Build.BinarySHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("digest does not match the running binary: %s", page.Build.BinarySHA256)
	}
}

// Build identity describes this process. A restart must report the new process,
// and nothing may leak into durable state where a later run would inherit it.
func TestBuildIdentityIsNotPersisted(t *testing.T) {
	dir := t.TempDir()
	first, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	firstStart := first.build.StartedAt
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(dir + "/state.json"); err == nil {
		if strings.Contains(string(data), firstStart) || strings.Contains(string(data), "binarySha256") {
			t.Fatal("build identity written to durable state")
		}
		var persisted map[string]any
		if err := json.Unmarshal(data, &persisted); err == nil {
			if _, ok := persisted["build"]; ok {
				t.Fatal("build key persisted")
			}
		}
	}
	second, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if second.build.StartedAt == "" {
		t.Fatal("restart reported no start time")
	}
	if second.build.BinarySHA256 != first.build.BinarySHA256 {
		t.Fatal("same binary reported different digests")
	}
}

// The start time comes from the supplied clock, not an ambient one.
func TestBuildIdentityUsesSuppliedClock(t *testing.T) {
	identity := newBuildIdentity(time.Unix(0, 0))
	if identity.StartedAt != "1970-01-01T00:00:00Z" {
		t.Fatalf("start time not derived from the supplied clock: %s", identity.StartedAt)
	}
}
