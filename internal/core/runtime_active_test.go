// SPDX-License-Identifier: MPL-2.0

package core

import "testing"

func TestRuntimeActiveTracksClaimMembershipAndCompletion(t *testing.T) {
	s, _, target := setup(t)
	joinWorkTeam(t, s, target, "product", "lead", "builder", "reviewer", "red")
	rpc(t, s, "lead", "tasks.assign", teamAssignment())
	d := s.data.Deliveries[len(s.data.Deliveries)-1]
	if s.RuntimeActive(d.ID) || s.RuntimeActive("missing") {
		t.Fatal("unclaimed work considered active")
	}
	claim(t, s, "builder")
	if !s.RuntimeActive(d.ID) {
		t.Fatal("claimed work not active")
	}
	rpc(t, s, "red", "teams.leave", map[string]any{"id": "product"})
	if s.RuntimeActive(d.ID) {
		t.Fatal("withdrawn reviewer did not invalidate active scope")
	}
	rpc(t, s, "red", "teams.join", map[string]any{"id": "product"})
	if !s.RuntimeActive(d.ID) {
		t.Fatal("core predicate no longer reflects current membership")
	}
	if err := s.Finish(d.ID, "root", "Result", nil); err != nil {
		t.Fatal(err)
	}
	if s.RuntimeActive(d.ID) {
		t.Fatal("completed delivery still active")
	}
	s.Close()
	if s.RuntimeActive(d.ID) {
		t.Fatal("closed service authorizes active work")
	}
}
