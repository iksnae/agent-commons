// SPDX-License-Identifier: MPL-2.0

package core

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// fixture carries the identifiers populate created, so each read-only method can
// be called with params that resolve a real record instead of erroring out on a
// missing one. A method that finds nothing exercises almost none of its handler,
// and would report a clean state comparison it never earned.
type fixture struct {
	target    string
	teamID    string
	contextID string
	postID    string
	taskID    string
}

// TestReadOnlyMethodsDoNotMutateState pins the classification at the point where
// it matters: a method listed in readOnlyMethods bypasses Service.mutate, so
// anything it writes into s.data is never saved and never rolled back, and then
// persists at whatever later moment an unrelated mutating call happens to save.
// Every listed method must therefore leave s.data byte-identical. Methods absent
// from the map are routed through mutate and are safe by construction, so they
// are deliberately not exercised here.
//
// State is compared through json.Marshal(s.data), the same representation mutate
// snapshots at service.go:196. So this test's blind spot -- state not visible
// through JSON -- coincides exactly with mutate's rollback blind spot, and it
// cannot miss a write that rollback would have restored.
func TestReadOnlyMethodsDoNotMutateState(t *testing.T) {
	if len(readOnlyMethods) == 0 {
		t.Fatal("readOnlyMethods is empty; this test would assert nothing")
	}
	s, _, target := setup(t)
	f := populate(t, s, target)

	// An error return still counts as unchanged state, but a method that only
	// ever errors proves nothing about its classification: a handler that
	// rejected its params never got far enough to write. Every listed method
	// must therefore execute cleanly at least once.
	executed := map[string]bool{}

	for method := range readOnlyMethods {
		for _, actor := range []string{"operator", "builder", "reviewer"} {
			raw, err := json.Marshal(provokingParams(method, actor, f))
			if err != nil {
				t.Fatal(err)
			}
			before, err := json.Marshal(s.data)
			if err != nil {
				t.Fatal(err)
			}
			_, callErr := s.Call(actor, method, raw)
			if callErr == nil {
				executed[method] = true
			}
			after, err := json.Marshal(s.data)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Fatalf("%s is listed read-only but mutated state when called by %s; the write bypasses mutate, so it is never saved and never rolled back (call error: %v)", method, actor, callErr)
			}
		}
	}

	for method := range readOnlyMethods {
		if !executed[method] {
			t.Errorf("%s never completed a successful call, so its unchanged-state result is vacuous; fix the fixture rather than trusting the pass", method)
		}
	}
}

// provokingParams builds params that resolve real records for the method under
// test and carry every lever that method could plausibly use to write. Only the
// operator may name a target explicitly; a session is pinned to its own.
func provokingParams(method, actor string, f fixture) params {
	p := params{
		Text:             "provoke",
		Title:            "provoke",
		Criteria:         "provoke",
		Evidence:         "provoke",
		Output:           "provoke",
		Verdict:          "accepted",
		To:               "reviewer",
		Reviewer:         "reviewer",
		RedTeam:          "red",
		IdempotencyKey:   "provoke-key",
		SessionID:        "builder",
		NativeID:         "native-one",
		LeaseID:          "lease-one",
		Epoch:            1,
		Limit:            10,
		Order:            "newest",
		Query:            "hello",
		Topic:            "technique",
		ExpectedVersion:  1,
		Version:          1,
		ExpectedRevision: 1,
		// Retirement arrived after this fixture and its lever was missed, so
		// sessions.list's retirement branch went unprovoked while the comment
		// above claimed every lever was set. Operator-only: for the other two
		// actors this provokes the refusal path instead, which must not write
		// either.
		IncludeRetired: true,
	}
	if actor == "operator" {
		p.Target = f.target
	}
	// Keyed by domain rather than by exact method, so that a method newly
	// added to readOnlyMethods is handed a resolvable ID too, instead of
	// erroring on a missing record and passing this test vacuously.
	switch {
	case strings.HasPrefix(method, "teams."):
		p.ID = f.teamID
	case strings.HasPrefix(method, "board."):
		p.ID = f.postID
	case strings.HasPrefix(method, "tasks."):
		p.ID = f.taskID
	case strings.HasPrefix(method, "context."):
		p.ID = f.contextID
	}
	return p
}

// populate gives the fixture real records for read-only handlers to walk, and
// somewhere for an accidental write to land.
func populate(t *testing.T, s *Service, target string) fixture {
	t.Helper()
	// Sessions are registered against the symlink-resolved target, so the
	// operator must name that same canonical path or scope checks reject it.
	canonical, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	f := fixture{target: canonical, teamID: "team-one", contextID: "ctx-one"}
	rpc(t, s, "operator", "teams.create", map[string]any{"id": f.teamID, "target": f.target, "title": "Team One", "text": "Team brief."})
	rpc(t, s, "operator", "teams.invite", map[string]any{"id": f.teamID, "target": f.target, "to": "builder"})
	rpc(t, s, "builder", "teams.join", map[string]any{"id": f.teamID})
	// reviewer is invited but deliberately left unjoined. A membership write
	// the fixture already performed is idempotent by the time the loop runs,
	// so teams.join would pass byte-equality on a no-op; the unconsumed
	// invitation keeps a real write available to provoke.
	rpc(t, s, "operator", "teams.invite", map[string]any{"id": f.teamID, "target": f.target, "to": "reviewer"})
	post := rpc(t, s, "lead", "board.post", map[string]any{"topic": "technique", "title": "Post One", "text": "Board body", "evidence": "live probe", "idempotencyKey": "populate-board"}).(BoardPost)
	f.postID = post.ID
	send(t, s, "lead", "builder", "populate-one")
	send(t, s, "builder", "lead", "populate-two")
	rpc(t, s, "lead", "context.put", map[string]any{"id": f.contextID, "text": "context body", "expectedVersion": 0})
	assigned := rpc(t, s, "lead", "tasks.assign", map[string]any{"to": "builder", "title": "Task One", "text": "inspect", "criteria": "evidence", "reviewer": "reviewer", "idempotencyKey": "populate-task"}).(Task)
	f.taskID = assigned.ID
	// A retired identity and an abandoned task, so the terminal shapes of both
	// ledgers are present. Without them the retirement branch of sessions.list
	// and every terminal-status branch a read-only handler walks are skipped
	// entirely, and their unchanged-state result is about records that were
	// never there.
	//
	// Both are extra records rather than edits to the ones above: f.taskID must
	// stay a live task, or the task handlers stop reaching the code that runs
	// for work still in flight.
	enrollAgent(t, s, "agent-retired", "exp-retired", "builder", f.target)
	rpc(t, s, "operator", "sessions.retire", map[string]any{"id": "agent-retired",
		"evidence": "Withdrawn so the fixture carries a tombstone to walk."})
	//
	// The abandoned task is submitted and rejected first, which is the state
	// abandonment exists for and leaves a record with an output and a verdict
	// on it. A bare assign-then-abandon carries empty fields, and a write into
	// an already-empty field is invisible to a byte comparison -- which a
	// mutation writing to Output on this branch demonstrated by surviving.
	// Submission goes through tasks.submit rather than Claim/Finish so the
	// fixture's delivery queue and the builder's busy flag stay as the cases
	// above found them.
	abandoned := rpc(t, s, "lead", "tasks.assign", map[string]any{"to": "builder", "title": "Task Two", "text": "inspect", "criteria": "evidence", "reviewer": "reviewer", "idempotencyKey": "populate-abandoned"}).(Task)
	abandoned = rpc(t, s, "builder", "tasks.submit", map[string]any{"id": abandoned.ID, "output": "Work that stopped here.", "expectedRevision": abandoned.Revision}).(Task)
	rpc(t, s, "reviewer", "tasks.review", map[string]any{"id": abandoned.ID, "verdict": "rejected", "evidence": "Not adequate.", "expectedRevision": abandoned.Revision})
	rpc(t, s, "operator", "tasks.abandon", map[string]any{"id": abandoned.ID,
		"evidence": "Stopped so the fixture carries a terminal task that was never accepted."})
	return f
}
