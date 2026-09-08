// SPDX-License-Identifier: MPL-2.0

package core

import (
	"encoding/json"
	"testing"
)

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

// TestEnrollFailsClosedOnMultipleCandidates covers the defect: the enroll
// resolution loop took the first matching candidate from a Go map, whose
// iteration order is randomized per process, so with two matching sessions
// enroll would silently bind to an arbitrary one and hand back THAT
// identity's live token. It must instead fail closed and name both
// candidate IDs, never pick one.
//
// Two blank-Name, same-target, same-role sessions are the ordinary
// multi-agent-per-role shape (see TestSessionsRegisterAllowsSharedRoleBlankNames
// below), so this state is constructed directly in s.data the same way the
// pre-existing tests in this package reach into Service internals — mirroring
// how sessions.register would actually produce it, canonicalizing the target
// first since a raw t.TempDir() string can mismatch the canonicalized form
// stored by the service (e.g. /var vs /private/var symlink resolution on
// macOS).
func TestEnrollFailsClosedOnMultipleCandidates(t *testing.T) {
	dir := t.TempDir()
	target := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	canonical, cErr := canonicalTarget(target)
	if cErr != nil {
		t.Fatal(cErr)
	}
	s.data.Sessions["dup-a"] = Session{ID: "dup-a", Target: canonical, Role: "worker", Mode: "manual", Runtime: "manual", Policy: "coordination"}
	s.data.Sessions["dup-b"] = Session{ID: "dup-b", Target: canonical, Role: "worker", Mode: "manual", Runtime: "manual", Policy: "coordination"}
	s.data.Tokens["dup-a"] = "tok-a"
	s.data.Tokens["dup-b"] = "tok-b"

	enroll := Session{ID: "fresh-hash", Role: "worker", Target: target, Mode: "manual", Runtime: "manual"}
	b, e := json.Marshal(enroll)
	if e != nil {
		t.Fatal(e)
	}
	out, callErr := s.Call("operator", "sessions.enroll", b)
	if callErr == nil {
		t.Fatalf("expected ambiguity error, got success: %#v", out)
	}
	if out != nil {
		t.Fatalf("expected no token/session returned on ambiguity error, got %#v", out)
	}
	if len(s.data.Sessions) != 2 {
		t.Fatalf("expected roster to stay at 2 entries, got %d", len(s.data.Sessions))
	}
	if _, ok := s.data.Sessions["fresh-hash"]; ok {
		t.Fatal("enroll must not mint a new session when candidates are ambiguous")
	}
}

// TestSessionsRegisterAllowsSharedRoleBlankNames is the architect's required
// regression: Target+Role is NOT a unique identity key. Two agents sharing a
// role on one target is the product's ordinary shape (mirrored by
// integration/return_path_test.go registering lead and worker both as
// coordinator, and internal/transport/transport_test.go registering alice
// and bob both as builder). Both register calls here must succeed so a
// future well-meaning guard cannot silently forbid this shape again.
func TestSessionsRegisterAllowsSharedRoleBlankNames(t *testing.T) {
	dir := t.TempDir()
	target := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	rpc(t, s, "operator", "sessions.register", Session{ID: "shared-a", Target: target, Role: "worker", Mode: "manual", Runtime: "manual"})
	rpc(t, s, "operator", "sessions.register", Session{ID: "shared-b", Target: target, Role: "worker", Mode: "manual", Runtime: "manual"})
	if len(s.data.Sessions) != 2 {
		t.Fatalf("expected both shared-role blank-Name registrations to succeed, got %d sessions", len(s.data.Sessions))
	}
}

// TestEnrollSingleCandidateStillAdoptsPilotShape guards against regression
// in the single-candidate case, using the live pilot roles' shape: blank
// Name, one session per target+role (coordinator, workspace-lead).
func TestEnrollSingleCandidateStillAdoptsPilotShape(t *testing.T) {
	dir := t.TempDir()
	target := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	coordReg := rpc(t, s, "operator", "sessions.register", Session{ID: "pilot-coordinator", Target: target, Role: "coordinator", Mode: "manual", Runtime: "manual"}).(map[string]any)
	leadReg := rpc(t, s, "operator", "sessions.register", Session{ID: "pilot-workspace-lead", Target: target, Role: "workspace-lead", Mode: "manual", Runtime: "manual"}).(map[string]any)
	coordToken := coordReg["token"]
	leadToken := leadReg["token"]

	coordEnroll := rpc(t, s, "operator", "sessions.enroll", Session{ID: "fresh-coord-hash", Role: "coordinator", Target: target, Mode: "manual", Runtime: "manual"}).(map[string]any)
	coordSession := coordEnroll["session"].(Session)
	if coordSession.ID != "pilot-coordinator" {
		t.Fatalf("expected adopt of pilot-coordinator, got %q", coordSession.ID)
	}
	if coordEnroll["token"] != coordToken {
		t.Fatal("expected enroll to preserve the existing coordinator token")
	}

	leadEnroll := rpc(t, s, "operator", "sessions.enroll", Session{ID: "fresh-lead-hash", Role: "workspace-lead", Target: target, Mode: "manual", Runtime: "manual"}).(map[string]any)
	leadSession := leadEnroll["session"].(Session)
	if leadSession.ID != "pilot-workspace-lead" {
		t.Fatalf("expected adopt of pilot-workspace-lead, got %q", leadSession.ID)
	}
	if leadEnroll["token"] != leadToken {
		t.Fatal("expected enroll to preserve the existing workspace-lead token")
	}

	if len(s.data.Sessions) != 2 {
		t.Fatalf("expected roster to stay at 2 entries after adopts, got %d", len(s.data.Sessions))
	}
}
