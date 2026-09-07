// SPDX-License-Identifier: MPL-2.0

package core

import "testing"

func joinWorkTeam(t *testing.T, s *Service, target, team string, members ...string) {
	t.Helper()
	rpc(t, s, "operator", "teams.create", map[string]any{"id": team, "target": target, "title": "Work team", "text": "Independent review required."})
	for _, member := range members {
		rpc(t, s, "operator", "teams.invite", map[string]any{"id": team, "target": target, "to": member})
		rpc(t, s, member, "teams.join", map[string]any{"id": team})
		// Complete the service invitation so later claims address test work.
		d := claim(t, s, member)
		if err := s.Finish(d.ID, "", "Joined", nil); err != nil {
			t.Fatal(err)
		}
	}
}

func teamAssignment() map[string]any {
	return map[string]any{"teamId": "product", "to": "builder", "title": "Inspect", "text": "Private work", "criteria": "Evidence", "reviewer": "reviewer", "redTeam": "red", "idempotencyKey": "work"}
}

func TestTeamTaskRequiresJoinedParticipantsAndIndependentReview(t *testing.T) {
	s, _, target := setup(t)
	joinWorkTeam(t, s, target, "product", "lead", "builder", "reviewer")
	args := teamAssignment()
	denied(t, s, "lead", "tasks.assign", args)
	joinWorkTeam(t, s, target, "product", "red")
	task := rpc(t, s, "lead", "tasks.assign", args).(Task)
	if task.TeamID != "product" || s.data.SchemaVersion != 3 {
		t.Fatal("team task scope or schema missing")
	}
	rpc(t, s, "operator", "sessions.register", Session{ID: "outsider", Target: target, Mode: "manual", Runtime: "codex", Policy: "workflow"})
	denied(t, s, "outsider", "tasks.get", map[string]any{"id": task.ID})
	if tasks := rpc(t, s, "outsider", "tasks.list", nil).([]Task); len(tasks) != 0 {
		t.Fatal("team task exposed through project listing")
	}
	rpc(t, s, "builder", "tasks.submit", map[string]any{"id": task.ID, "output": "Private result", "expectedRevision": 0})
	denied(t, s, "builder", "tasks.review", map[string]any{"id": task.ID, "verdict": "approved", "evidence": "Self", "expectedRevision": 1})
	for _, reviewer := range []string{"reviewer", "red"} {
		rpc(t, s, reviewer, "tasks.review", map[string]any{"id": task.ID, "verdict": "approved", "evidence": "Independent evidence", "expectedRevision": 1})
	}
	rpc(t, s, "lead", "tasks.accept", map[string]any{"id": task.ID})
	for _, d := range s.data.Deliveries {
		if d.TaskID == task.ID && d.TeamID != task.TeamID {
			t.Fatal("task result lost team scope")
		}
	}
}

func TestTeamRevocationStopsAccessClaimAndIdempotentReplay(t *testing.T) {
	s, dir, target := setup(t)
	joinWorkTeam(t, s, target, "product", "lead", "builder", "reviewer", "red")
	task := rpc(t, s, "lead", "tasks.assign", teamAssignment()).(Task)
	rpc(t, s, "operator", "teams.revoke", map[string]any{"id": "product", "target": target, "to": "builder"})
	denied(t, s, "builder", "tasks.get", map[string]any{"id": task.ID})
	denied(t, s, "builder", "tasks.submit", map[string]any{"id": task.ID, "output": "Revoked", "expectedRevision": 0})
	if d, err := s.Claim("builder"); err != nil || d != nil {
		t.Fatal("revoked team work claimed", err)
	}
	for _, d := range rpc(t, s, "builder", "inbox.list", nil).([]Delivery) {
		if d.TaskID == task.ID {
			t.Fatal("revoked team work exposed in inbox")
		}
	}
	args := teamAssignment()
	delete(args, "teamId")
	denied(t, s, "lead", "tasks.assign", args)
	if s.data.Tasks[task.ID].Status != "assigned" {
		t.Fatal("revocation rewrote retained task evidence")
	}
	s.Close()
	reopened, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	denied(t, reopened, "builder", "tasks.get", map[string]any{"id": task.ID})
	if d, err := reopened.Claim("builder"); err != nil || d != nil {
		t.Fatal("restart bypassed revoked team membership", err)
	}
}

func TestTeamChangeDuringExecutionWithholdsResult(t *testing.T) {
	s, _, target := setup(t)
	joinWorkTeam(t, s, target, "product", "lead", "builder", "reviewer", "red")
	task := rpc(t, s, "lead", "tasks.assign", teamAssignment()).(Task)
	d := claim(t, s, "builder")
	rpc(t, s, "reviewer", "teams.leave", map[string]any{"id": "product"})
	before := len(s.data.Deliveries)
	if err := s.Finish(d.ID, "native-root", "Must not submit", nil); err != nil {
		t.Fatal(err)
	}
	if got := s.data.Tasks[task.ID]; got.Status != "failed" || got.Output != "" || got.Revision != 0 || len(s.data.Deliveries) != before {
		t.Fatal("team change allowed task progression or result delivery")
	}
}
