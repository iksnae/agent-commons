// SPDX-License-Identifier: MPL-2.0

package core

import (
	"encoding/json"
	"strings"
	"testing"
)

func reinstateArgs(id, reason string) map[string]any {
	return map[string]any{"id": id, "evidence": reason}
}

// The contract is "retirement, not deletion. Nothing is erased." Clearing
// RetiredAt and RetiredReason on reinstatement erased the retirement itself,
// which made this the one evidence-bearing method whose evidence was demanded
// and then discarded. The history outlives both halves of the cycle, and the
// cycle can repeat without loss.
func TestRetirementHistoryOutlivesReinstatement(t *testing.T) {
	s, _, target := retireFixture(t)
	enrollAgent(t, s, "agent-alpha", "exp-alpha", "builder", target)

	rpc(t, s, "operator", "sessions.retire", map[string]any{"id": "agent-alpha", "evidence": "First withdrawal: the communications test ended."})
	afterFirstRetire := s.data.Sessions["agent-alpha"]
	if len(afterFirstRetire.Retirements) != 1 {
		t.Fatalf("retirement recorded %d history entries, want 1", len(afterFirstRetire.Retirements))
	}
	open := afterFirstRetire.Retirements[0]
	if open.At != afterFirstRetire.RetiredAt || open.Reason != afterFirstRetire.RetiredReason {
		t.Fatalf("history entry disagrees with current state: %+v vs %q/%q", open, afterFirstRetire.RetiredAt, afterFirstRetire.RetiredReason)
	}
	if open.ReinstatedAt != "" || open.ReinstatedReason != "" {
		t.Fatalf("a fresh retirement is already closed: %+v", open)
	}

	rpc(t, s, "operator", "sessions.reinstate", reinstateArgs("agent-alpha", "First return: the experiment resumed."))
	afterFirstReinstate := s.data.Sessions["agent-alpha"]
	if afterFirstReinstate.RetiredAt != "" || afterFirstReinstate.RetiredReason != "" {
		t.Fatalf("reinstatement left current state retired: %+v", afterFirstReinstate)
	}
	if len(afterFirstReinstate.Retirements) != 1 {
		t.Fatalf("reinstatement changed the history length to %d", len(afterFirstReinstate.Retirements))
	}
	closed := afterFirstReinstate.Retirements[0]
	if closed.At != open.At || closed.Reason != open.Reason {
		t.Fatalf("reinstatement rewrote the retirement it closed: %+v -> %+v", open, closed)
	}
	if closed.ReinstatedAt == "" {
		t.Fatal("reinstatement recorded no timestamp")
	}
	if closed.ReinstatedReason != "First return: the experiment resumed." {
		t.Fatalf("reinstatement reason is %q; the evidence it demanded was discarded", closed.ReinstatedReason)
	}

	// A second cycle appends rather than overwriting, or the history is a
	// single slot pretending to be a ledger.
	rpc(t, s, "operator", "sessions.retire", map[string]any{"id": "agent-alpha", "evidence": "Second withdrawal: wrong role after all."})
	rpc(t, s, "operator", "sessions.reinstate", reinstateArgs("agent-alpha", "Second return: role corrected upstream."))
	history := s.data.Sessions["agent-alpha"].Retirements
	if len(history) != 2 {
		t.Fatalf("two cycles produced %d entries, want 2", len(history))
	}
	if history[0] != closed {
		t.Fatalf("the second cycle rewrote the first: %+v -> %+v", closed, history[0])
	}
	if history[1].Reason != "Second withdrawal: wrong role after all." || history[1].ReinstatedReason != "Second return: role corrected upstream." {
		t.Fatalf("second cycle recorded %+v", history[1])
	}
	if history[1].At <= history[0].At {
		t.Fatalf("history is not in order: %q then %q", history[0].At, history[1].At)
	}
}

// Retirement prose is operator evidence about an identity, not something that
// identity's peers are owed. Before history persisted, no agent could ever see
// it -- retired sessions are excluded from sessions.list and includeRetired is
// operator-only. A reinstated session IS listed to every peer scoped to its
// target, so keeping the history past reinstatement is exactly what would have
// leaked it.
func TestRetirementHistoryIsNeverShownToAPeer(t *testing.T) {
	s, _, target := retireFixture(t)
	enrollAgent(t, s, "agent-alpha", "exp-alpha", "builder", target)
	const withdrawal = "Operator-only prose about why this identity was withdrawn."
	const restoration = "Operator-only prose about why it came back."
	rpc(t, s, "operator", "sessions.retire", map[string]any{"id": "agent-alpha", "evidence": withdrawal})
	rpc(t, s, "operator", "sessions.reinstate", reinstateArgs("agent-alpha", restoration))

	peerListing := rpc(t, s, "lead", "sessions.list", map[string]any{}).([]Session)
	var listed *Session
	for i := range peerListing {
		if peerListing[i].ID == "agent-alpha" {
			listed = &peerListing[i]
		}
	}
	if listed == nil {
		t.Fatal("a reinstated identity vanished from its peers' listing")
	}
	if len(listed.Retirements) != 0 {
		t.Fatalf("peer received retirement history: %+v", listed.Retirements)
	}
	// Assert on the bytes a peer would actually receive, not only on the field:
	// a helper that clears the slice on one path and not another still leaks.
	encoded := string(mustJSON(t, peerListing))
	for _, secret := range []string{withdrawal, restoration, "retirements"} {
		if strings.Contains(encoded, secret) {
			t.Fatalf("peer-facing session JSON carries %q: %s", secret, encoded)
		}
	}

	// The operator, who owns the evidence, still gets it.
	operatorListing := rpc(t, s, "operator", "sessions.list", map[string]any{}).([]Session)
	found := false
	for _, v := range operatorListing {
		if v.ID == "agent-alpha" {
			found = true
			if len(v.Retirements) != 1 || v.Retirements[0].Reason != withdrawal {
				t.Fatalf("operator lost the history too: %+v", v.Retirements)
			}
		}
	}
	if !found {
		t.Fatal("reinstated identity missing from the operator listing")
	}

	// The DTO must not alias the record, or a caller could edit the ledger.
	// Clearing the slice HEADER on the copy would prove nothing -- that never
	// reaches the original whether or not the backing array is shared -- so
	// this writes through to an element.
	var mine *Session
	for i := range operatorListing {
		if operatorListing[i].ID == "agent-alpha" {
			mine = &operatorListing[i]
		}
	}
	if mine == nil || len(mine.Retirements) != 1 {
		t.Fatalf("operator listing lost the entry under test: %+v", mine)
	}
	mine.Retirements[0].Reason = "tampered through the returned DTO"
	if stored := s.data.Sessions["agent-alpha"].Retirements[0].Reason; stored != withdrawal {
		t.Fatalf("the returned DTO aliased the stored ledger: it now reads %q", stored)
	}
}

// The supervisor is not the operator either. Sessions feeds the runtime poller,
// which has no business carrying operator evidence into a runtime prompt.
func TestSupervisorSnapshotCarriesNoRetirementHistory(t *testing.T) {
	s, _, target := retireFixture(t)
	enrollAgent(t, s, "agent-alpha", "exp-alpha", "builder", target)
	rpc(t, s, "operator", "sessions.retire", map[string]any{"id": "agent-alpha", "evidence": "Withdrawn for the supervisor's benefit."})
	rpc(t, s, "operator", "sessions.reinstate", reinstateArgs("agent-alpha", "Back again."))
	for _, v := range s.Sessions() {
		if len(v.Retirements) != 0 {
			t.Fatalf("Sessions() exposed retirement history for %s: %+v", v.ID, v.Retirements)
		}
	}
	if len(s.data.Sessions["agent-alpha"].Retirements) != 1 {
		t.Fatal("Sessions() stripped the stored history rather than its copy")
	}
}

// reopenWithCorruptedState applies a hand edit to saved state and returns the
// error loading it produces, or "" if it loaded. Each caller names one specific
// corruption, so a refusal cannot be credited to the wrong invariant.
func reopenWithCorruptedState(t *testing.T, s *Service, dir string, corrupt func(*Service)) string {
	t.Helper()
	corrupt(s)
	if err := s.save(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(dir)
	if err == nil {
		reopened.Close()
		return ""
	}
	return err.Error()
}

// A retirement record on a schema that predates the feature is a hand edit, and
// the older binary that wrote that schema would enforce none of the rules the
// record implies. This is the schema-floor half of the load guard, which the
// team guard has too.
func TestLoadRefusesRetirementRecordsBelowSchemaFive(t *testing.T) {
	s, dir, target := retireFixture(t)
	enrollAgent(t, s, "agent-alpha", "exp-alpha", "builder", target)
	if s.data.SchemaVersion >= retirementSchema {
		t.Fatalf("fixture is already at schema %d; this test needs an older one", s.data.SchemaVersion)
	}
	message := reopenWithCorruptedState(t, s, dir, func(s *Service) {
		v := s.data.Sessions["agent-alpha"]
		v.RetiredAt = "2026-09-08T00:00:00Z"
		v.RetiredReason = "Backdated onto a schema that never knew about retirement."
		v.Retirements = []Retirement{{At: v.RetiredAt, Reason: v.RetiredReason}}
		s.data.Sessions["agent-alpha"] = v
		delete(s.data.Tokens, "agent-alpha")
	})
	if message == "" {
		t.Fatal("a retirement record loaded on a pre-retirement schema")
	}
	if !strings.Contains(message, "schema 5") {
		t.Fatalf("refusal does not name the schema floor it enforces: %q", message)
	}
}

// A tombstone with no reason is not a record of anything. The evidence is
// required at the method, so a saved one without it was edited by hand.
func TestLoadRefusesATombstoneMissingItsReason(t *testing.T) {
	s, dir, target := retireFixture(t)
	enrollAgent(t, s, "agent-alpha", "exp-alpha", "builder", target)
	rpc(t, s, "operator", "sessions.retire", retireArgs("agent-alpha"))
	// Only CURRENT STATE is corrupted, and the ledger entry keeps a real
	// reason. Blanking both would be refused by the ledger-entry check instead,
	// and this test would pass without the branch it is named for existing --
	// which is exactly what a mutation showed it doing.
	message := reopenWithCorruptedState(t, s, dir, func(s *Service) {
		v := s.data.Sessions["agent-alpha"]
		v.RetiredReason = "   "
		s.data.Sessions["agent-alpha"] = v
	})
	if message == "" {
		t.Fatal("a tombstone with no reason loaded")
	}
	// The exact message, because several other invariants would also refuse
	// this state and "contains reason" cannot tell them apart.
	if !strings.Contains(message, "retired identity is missing its reason") {
		t.Fatalf("refusal came from a different invariant than the missing reason: %q", message)
	}
}

// Current state and history are two representations of one fact, so they are
// made to agree by validation rather than by hoping both writers stay correct.
//
// Every subtest names the exact message it expects. Asserting only that loading
// FAILED cannot tell nine invariants apart, and a table that reads as nine
// branches then quietly exercises three -- which is what the previous version
// of this test did, with two of its four cases driving the same branch.
//
// The fixture is two full cycles (retire, reinstate, retire), so the ledger has
// one closed entry and one open one and every branch is reachable by editing it.
func TestLoadRefusesHistoryThatDisagreesWithCurrentState(t *testing.T) {
	for _, corruption := range []struct {
		name    string
		want    string
		corrupt func(*Service)
	}{
		{"entry with no reason", "retirement history entry is missing its timestamp or reason",
			func(s *Service) { editLedger(s, func(h []Retirement) { h[0].Reason = "  " }) }},
		{"entry with no timestamp", "retirement history entry is missing its timestamp or reason",
			func(s *Service) { editLedger(s, func(h []Retirement) { h[0].At = "" }) }},
		{"reinstatement reason with no timestamp", "retirement history records a reinstatement reason with no timestamp",
			func(s *Service) { editLedger(s, func(h []Retirement) { h[0].ReinstatedAt = "" }) }},
		// R23c is load-bearing: without it a second open entry loads, "the open
		// one" silently resolves to the last, and the earlier entry stays open
		// forever while reinstate only ever closes the newest.
		{"more than one open entry", "retirement history has more than one open entry",
			func(s *Service) {
				editLedger(s, func(h []Retirement) { h[0].ReinstatedAt, h[0].ReinstatedReason = "", "" })
			}},
		// Two routes to one invariant, kept apart and labelled as such rather
		// than passed off as two branches.
		{"retired but the final entry is closed", "retired identity has no open retirement history entry",
			func(s *Service) {
				editLedger(s, func(h []Retirement) {
					h[len(h)-1].ReinstatedAt = "2026-09-08T00:00:00Z"
					h[len(h)-1].ReinstatedReason = "Closed while still retired."
				})
			}},
		{"retired with no history at all", "retired identity has no open retirement history entry",
			func(s *Service) { editLedger(s, func(h []Retirement) {}); clearLedger(s) }},
		{"open entry timestamp does not match", "retirement history disagrees with the current retirement",
			func(s *Service) { editLedger(s, func(h []Retirement) { h[len(h)-1].At = "2020-01-01T00:00:00Z" }) }},
		{"open entry reason does not match", "retirement history disagrees with the current retirement",
			func(s *Service) {
				editLedger(s, func(h []Retirement) { h[len(h)-1].Reason = "A reason nobody gave." })
			}},
		{"live identity left with an open entry", "live identity has an open retirement history entry",
			func(s *Service) {
				v := s.data.Sessions["agent-alpha"]
				v.RetiredAt, v.RetiredReason = "", ""
				s.data.Sessions["agent-alpha"] = v
				s.data.Tokens["agent-alpha"] = randomID()
			}},
		{"reason with no timestamp", "retirement reason recorded without a timestamp",
			func(s *Service) {
				v := s.data.Sessions["agent-alpha"]
				v.RetiredAt = ""
				s.data.Sessions["agent-alpha"] = v
			}},
	} {
		t.Run(corruption.name, func(t *testing.T) {
			s, dir, target := retireFixture(t)
			enrollAgent(t, s, "agent-alpha", "exp-alpha", "builder", target)
			rpc(t, s, "operator", "sessions.retire", retireArgs("agent-alpha"))
			rpc(t, s, "operator", "sessions.reinstate", reinstateArgs("agent-alpha", "Back once."))
			rpc(t, s, "operator", "sessions.retire", retireArgs("agent-alpha"))
			if got := len(s.data.Sessions["agent-alpha"].Retirements); got != 2 {
				t.Fatalf("fixture ledger has %d entries, want 2", got)
			}
			message := reopenWithCorruptedState(t, s, dir, corruption.corrupt)
			if message == "" {
				t.Fatal("inconsistent retirement history loaded")
			}
			if !strings.Contains(message, corruption.want) {
				t.Fatalf("refused by a different invariant than the one under test:\n got %q\nwant %q", message, corruption.want)
			}
		})
	}
}

// editLedger applies a hand edit to the stored ledger. It takes the slice so a
// corruption can address entries by position, which is how the open one and the
// closed one are told apart.
func editLedger(s *Service, edit func([]Retirement)) {
	v := s.data.Sessions["agent-alpha"]
	edit(v.Retirements)
	s.data.Sessions["agent-alpha"] = v
}

func clearLedger(s *Service) {
	v := s.data.Sessions["agent-alpha"]
	v.Retirements = nil
	s.data.Sessions["agent-alpha"] = v
}

// The explicit-id route to the adopt branch. The candidate scan matches on
// target + role + name, so an --id naming a retired record of a DIFFERENT role
// reaches the adopt branch without ever being classified. Both paths refuse,
// but only this one tells the operator the identity is retired instead of the
// generic conflict message, and only a test keeps that true.
func TestEnrollWithAnExplicitIdNamesTheRetirementItLandedOn(t *testing.T) {
	s, _, target := retireFixture(t)
	enrollAgent(t, s, "agent-alpha", "exp-alpha", "reviewer", target)
	rpc(t, s, "operator", "sessions.retire", retireArgs("agent-alpha"))

	// Same id, same name, different role: no candidate matches, so resolution
	// leaves the client's id alone and the adopt branch receives the tombstone.
	raw := mustJSON(t, Session{ID: "agent-alpha", Name: "exp-alpha", Role: "builder",
		Target: target, Runtime: "manual", Mode: "manual", Policy: "coordination"})
	_, err := s.Call("operator", "sessions.enroll", raw)
	if err == nil {
		t.Fatal("enroll adopted a retired record named by an explicit id")
	}
	for _, want := range []string{"agent-alpha", "retired", "sessions.reinstate"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("refusal omits %q, so the operator is not told the identity is retired: %q", want, err)
		}
	}
	if _, ok := s.data.Tokens["agent-alpha"]; ok {
		t.Fatal("the refused enroll issued a credential")
	}
	if s.data.Sessions["agent-alpha"].RetiredAt == "" {
		t.Fatal("the refused enroll cleared the tombstone")
	}
}

// A retired identity's history must survive a restart intact, or the ledger is
// only a runtime convenience.
func TestRetirementHistorySurvivesRestart(t *testing.T) {
	s, dir, target := retireFixture(t)
	enrollAgent(t, s, "agent-alpha", "exp-alpha", "builder", target)
	rpc(t, s, "operator", "sessions.retire", retireArgs("agent-alpha"))
	rpc(t, s, "operator", "sessions.reinstate", reinstateArgs("agent-alpha", "Returned before the restart."))
	before := mustJSON(t, s.data.Sessions["agent-alpha"].Retirements)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	after := mustJSON(t, reopened.data.Sessions["agent-alpha"].Retirements)
	if string(after) != string(before) {
		t.Fatalf("restart altered the retirement ledger: %s -> %s", before, after)
	}
	var decoded []Retirement
	if err := json.Unmarshal(after, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded) != 1 || decoded[0].ReinstatedReason != "Returned before the restart." {
		t.Fatalf("ledger did not survive as written: %+v", decoded)
	}
}

// reinstate's open-entry check cannot be reached through the API -- retire
// always appends an open entry, and validateRetirement refuses to LOAD a
// tombstone without one -- so it is defence in depth, and its failure mode is
// what makes it worth pinning: without the n == 0 clause the method indexes
// history[-1] and panics. A panic in a coordination service is not a refusal,
// it takes the whole process down with the lock held.
//
// Both cases corrupt state in memory only. Saving them and reopening would be
// refused by validateRetirement instead, and this test would then pass without
// the branch it names existing.
func TestReinstateRefusesRatherThanPanicsOnAnUnusableLedger(t *testing.T) {
	for _, c := range []struct {
		name    string
		corrupt func(Session) Session
	}{
		{"no history at all", func(v Session) Session {
			v.Retirements = nil
			return v
		}},
		{"history whose last entry is already closed", func(v Session) Session {
			v.Retirements[len(v.Retirements)-1].ReinstatedAt = "2026-09-08T00:00:00Z"
			v.Retirements[len(v.Retirements)-1].ReinstatedReason = "Closed by something other than reinstate."
			return v
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			s, _, target := retireFixture(t)
			enrollAgent(t, s, "agent-alpha", "exp-alpha", "builder", target)
			rpc(t, s, "operator", "sessions.retire", retireArgs("agent-alpha"))
			s.data.Sessions["agent-alpha"] = c.corrupt(s.data.Sessions["agent-alpha"])

			deniedWith(t, s, "operator", "sessions.reinstate",
				"retired identity has no open retirement history entry",
				reinstateArgs("agent-alpha", "Attempting to return an identity whose ledger cannot carry it."))
			// A refusal that quietly restored the credential would be worse
			// than the panic it replaced.
			if _, ok := s.data.Tokens["agent-alpha"]; ok {
				t.Fatal("the refused reinstatement issued a credential")
			}
			if s.data.Sessions["agent-alpha"].RetiredAt == "" {
				t.Fatal("the refused reinstatement cleared the tombstone")
			}
		})
	}
}
