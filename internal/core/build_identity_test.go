// SPDX-License-Identifier: MPL-2.0

package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"runtime/debug"
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

// The digest identifies a BUILD, not CODE: a commit touching only a test file
// still changes it, because plain `go build` stamps VCS info by default. That
// is why VCSRevision exists — it answers "same code", which the digest cannot.
//
// This exercises vcsIdentity directly against settings shaped like the real
// `go build ./cmd/agent-commons` output (AGENTS.md:30), verified by hand via
// `go build -o /tmp/actest ./cmd/agent-commons && go version -m /tmp/actest`,
// which reports "build vcs=git", "build vcs.revision=...", "build
// vcs.time=...", "build vcs.modified=true". It is deliberately NOT exercised
// through this test binary's own debug.ReadBuildInfo(): measured directly (`go
// test -c -o /tmp/t ./internal/core && go version -m /tmp/t`), a plain `go
// test` build carries no vcs.* settings at all — the test binary's synthetic
// main lives outside the module's directory, which fails the same-repository
// check `go help build` documents for -buildvcs=auto. So "the dev and test
// build path" is not one path: only `go build` stamps by default.
func TestBuildIdentityReportsVCSWhenStamped(t *testing.T) {
	revision, vcsTime, modified := vcsIdentity([]debug.BuildSetting{
		{Key: "-buildmode", Value: "exe"},
		{Key: "vcs", Value: "git"},
		{Key: "vcs.revision", Value: "bd232f2d2dd74a5b5e8d4aeb714cf627c74e2877"},
		{Key: "vcs.time", Value: "2026-09-08T12:12:50Z"},
		{Key: "vcs.modified", Value: "true"},
	})
	if revision != "bd232f2d2dd74a5b5e8d4aeb714cf627c74e2877" {
		t.Fatalf("VCSRevision not extracted: %q", revision)
	}
	if vcsTime != "2026-09-08T12:12:50Z" {
		t.Fatalf("VCSTime not extracted: %q", vcsTime)
	}
	if modified == nil || !*modified {
		t.Fatalf("VCSModified not extracted as true: %v", modified)
	}
}

// When a binary carries no vcs.* build settings at all (e.g. a release built
// with -buildvcs=false), all three VCS fields must be absent from the
// marshalled JSON, not merely zero-valued in the struct.
func TestBuildIdentityVCSFieldsAbsentWhenUnstamped(t *testing.T) {
	revision, vcsTime, modified := vcsIdentity(nil)
	if revision != "" || vcsTime != "" || modified != nil {
		t.Fatalf("expected genuine absence from an empty settings slice, got revision=%q time=%q modified=%v", revision, vcsTime, modified)
	}
	identity := BuildIdentity{StartedAt: "1970-01-01T00:00:00Z", VCSRevision: revision, VCSTime: vcsTime, VCSModified: modified}
	data, err := json.Marshal(identity)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"vcsRevision", "vcsTime", "vcsModified"} {
		if _, present := fields[key]; present {
			t.Fatalf("%s present in JSON though unstamped: %s", key, data)
		}
	}
}

// A false VCSModified is a real, stamped answer ("tree was clean") and must
// serialise, not vanish. A plain bool with omitempty would drop it here,
// making it indistinguishable from "never stamped" — the exact ambiguity
// VCSModified exists to remove.
func TestBuildIdentityVCSModifiedFalseSerialises(t *testing.T) {
	clean := false
	identity := BuildIdentity{StartedAt: "1970-01-01T00:00:00Z", VCSModified: &clean}
	data, err := json.Marshal(identity)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"vcsModified":false`) {
		t.Fatalf("vcsModified:false did not serialise: %s", data)
	}
}

// A caller asking "what is this service running?" must get a release name, not
// only a content hash it cannot map to a release. The digest answers "is this
// the binary I released"; Version answers "which release is that".
func TestBuildIdentityReportsTheStampedVersion(t *testing.T) {
	original := Version
	Version = "v1.2.3"
	t.Cleanup(func() { Version = original })
	if identity := newBuildIdentity(time.Unix(0, 0)); identity.Version != "v1.2.3" {
		t.Fatalf("stamped version not reported: %q", identity.Version)
	}
}

// An unstamped build says "dev" and never invents a release number. `go test`
// passes no -X flag, so this is what a developer's local binary reports.
func TestBuildIdentityReportsDevWhenUnstamped(t *testing.T) {
	if Version != DevVersion {
		t.Fatalf("test binary is stamped %q; the unstamped default is the thing under test", Version)
	}
	if identity := newBuildIdentity(time.Unix(0, 0)); identity.Version != "dev" {
		t.Fatalf("unstamped build reported %q, want dev", identity.Version)
	}
}

// Version is never absent from the wire. Unlike the vcs.* fields, "dev" is
// itself an answer, so a caller must never have to distinguish an omitted key
// from an unstamped build.
func TestBuildIdentityVersionAlwaysSerialises(t *testing.T) {
	data, err := json.Marshal(BuildIdentity{StartedAt: "1970-01-01T00:00:00Z", Version: DevVersion})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"version":"dev"`) {
		t.Fatalf("version did not serialise: %s", data)
	}
}

// runtime.status is the DTO agents read. The version must reach it, and the
// fields that were already there must be undisturbed.
func TestRuntimeStatusReportsTheBuildVersion(t *testing.T) {
	s, _, _ := setup(t)
	page := rpc(t, s, "operator", "runtime.status", map[string]any{}).(RuntimeStatusPage)
	if page.Build.Version != Version {
		t.Fatalf("runtime.status build.version = %q, want the running binary's %q", page.Build.Version, Version)
	}
	data, err := json.Marshal(page.Build)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if fields["version"] != Version {
		t.Fatalf("wire build.version = %v, want %q: %s", fields["version"], Version, data)
	}
	for _, key := range []string{"binarySha256", "startedAt"} {
		if _, present := fields[key]; !present {
			t.Fatalf("adding version disturbed the existing shape: %s missing: %s", key, data)
		}
	}
}
