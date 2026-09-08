// SPDX-License-Identifier: MPL-2.0

package core

import "testing"

// These tests cover the defect where sessions.enroll minted a second
// identity for a role already registered through the legacy operator
// sessions.register path. The client derives its own session ID with no
// server lookup, so a legacy record (created with an empty Name) never
// matched on ID and the duplicate-name guard never fired because it only
// compares non-empty names. The fix resolves by target+role+name-compatible
// match, server-side, before ID equality is even considered.

func TestEnrollAdoptsLegacyOperatorRegisteredSessionByIdentity(t *testing.T) {
	dir := t.TempDir()
	target := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	legacy := Session{ID: "legacy-hash-a", Target: target, Role: "worker", Mode: "manual", Runtime: "manual"}
	reg := rpc(t, s, "operator", "sessions.register", legacy).(map[string]any)
	legacyToken := reg["token"]

	if len(s.data.Sessions) != 1 {
		t.Fatalf("expected 1 session after legacy register, got %d", len(s.data.Sessions))
	}

	enroll := Session{ID: "legacy-hash-b", Name: "publisher-lead", Role: "worker", Target: target, Mode: "manual", Runtime: "manual"}
	out := rpc(t, s, "operator", "sessions.enroll", enroll).(map[string]any)

	adopted := out["session"].(Session)
	if adopted.ID != "legacy-hash-a" {
		t.Fatalf("expected enroll to adopt existing session ID legacy-hash-a, got %q", adopted.ID)
	}
	if out["token"] != legacyToken {
		t.Fatalf("expected enroll to preserve the existing token, got a reissued one")
	}
	if adopted.Name != "publisher-lead" {
		t.Fatalf("expected Name to be backfilled to publisher-lead, got %q", adopted.Name)
	}
	if len(s.data.Sessions) != 1 {
		t.Fatalf("expected roster to stay at 1 entry after adopt, got %d", len(s.data.Sessions))
	}
	if _, ok := s.data.Sessions["legacy-hash-b"]; ok {
		t.Fatal("enroll must not mint a second session under the client-derived ID")
	}
	stored := s.data.Sessions["legacy-hash-a"]
	if stored.Name != "publisher-lead" {
		t.Fatalf("stored session Name not backfilled, got %q", stored.Name)
	}
}

func TestEnrollNewRoleStillEnrollsNormally(t *testing.T) {
	dir := t.TempDir()
	target := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	rpc(t, s, "operator", "sessions.register", Session{ID: "legacy-a", Target: target, Role: "worker", Mode: "manual", Runtime: "manual"})

	fresh := Session{ID: "fresh-hash", Name: "new-role", Role: "reviewer", Target: target, Mode: "manual", Runtime: "manual"}
	out := rpc(t, s, "operator", "sessions.enroll", fresh).(map[string]any)
	created := out["session"].(Session)
	if created.ID != "fresh-hash" {
		t.Fatalf("expected a genuinely new role to enroll under its own ID, got %q", created.ID)
	}
	if len(s.data.Sessions) != 2 {
		t.Fatalf("expected roster to grow to 2 entries, got %d", len(s.data.Sessions))
	}
}

func TestSessionsRegisterStillRejectsNameCollisions(t *testing.T) {
	dir := t.TempDir()
	target := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	rpc(t, s, "operator", "sessions.register", Session{ID: "first", Name: "lead", Role: "lead", Target: target, Mode: "manual", Runtime: "manual"})
	denied(t, s, "operator", "sessions.register", Session{ID: "second", Name: "lead", Role: "lead", Target: target, Mode: "manual", Runtime: "manual"})
	if len(s.data.Sessions) != 1 {
		t.Fatalf("expected register name collision to be rejected outright, got %d sessions", len(s.data.Sessions))
	}
}

func TestEnrollMismatchedFieldsFailReconciliationInsteadOfAdopting(t *testing.T) {
	dir := t.TempDir()
	target := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	rpc(t, s, "operator", "sessions.register", Session{ID: "legacy-a", Target: target, Role: "worker", Team: "alpha", Mode: "manual", Runtime: "manual"})

	// Same target+role+name-compat (empty legacy Name) but mismatched Team,
	// so it must fail reconciliation, not adopt and not silently mint a
	// second session either.
	mismatch := Session{ID: "legacy-hash-c", Name: "publisher-lead", Role: "worker", Team: "beta", Target: target, Mode: "manual", Runtime: "manual"}
	denied(t, s, "operator", "sessions.enroll", mismatch)
	if len(s.data.Sessions) != 1 {
		t.Fatalf("expected mismatched reconciliation to fail without creating a new session, got %d", len(s.data.Sessions))
	}
}
