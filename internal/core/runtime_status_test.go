// SPDX-License-Identifier: MPL-2.0

package core

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestRuntimeStatusScopesCountsWithoutPrivateContents(t *testing.T) {
	s, _, target := setup(t)
	joinWorkTeam(t, s, target, "product", "lead", "builder", "reviewer", "red")
	args := teamAssignment()
	args["text"] = "private-message-sentinel"
	rpc(t, s, "lead", "tasks.assign", args)
	status := func() RuntimeQueueStatus {
		page := rpc(t, s, "builder", "runtime.status", nil).(RuntimeStatusPage)
		if len(page.Sessions) != 1 || page.Sessions[0].Identity != "builder" {
			t.Fatal("role saw another identity's status")
		}
		return page.Sessions[0]
	}
	if status().Ready != 1 {
		t.Fatal("claimable work missing")
	}
	rpc(t, s, "reviewer", "teams.leave", map[string]any{"id": "product"})
	if got := status(); got.Ready != 0 || got.WaitingTeam != 1 {
		t.Fatal("team-blocked work reported ready")
	}
	rpc(t, s, "reviewer", "teams.join", map[string]any{"id": "product"})
	d := claim(t, s, "builder")
	if got := status(); got.Running != 1 || !got.Busy {
		t.Fatal("active claim missing")
	}
	if err := s.Finish(d.ID, "private-native-root", "", ErrRuntimeCanceled); err != nil {
		t.Fatal(err)
	}
	if got := status(); got.Failed != 1 || got.Canceled != 1 || got.Busy || !got.NativeBound {
		t.Fatal("cancellation counts or binding observation wrong")
	}
	denied(t, s, "lead", "runtime.status", map[string]any{"sessionId": "builder"})
	denied(t, s, "lead", "runtime.status", map[string]any{"target": "/another-project"})
	before, _ := json.Marshal(s.data)
	page := rpc(t, s, "operator", "runtime.status", map[string]any{"sessionId": "builder"}).(RuntimeStatusPage)
	encoded, _ := json.Marshal(page)
	token, _ := s.Token("builder")
	for _, private := range []string{"private-message-sentinel", "private-native-root", token, d.ID} {
		if bytes.Contains(encoded, []byte(private)) {
			t.Fatal("health report exposed private content")
		}
	}
	after, _ := json.Marshal(s.data)
	if !bytes.Equal(before, after) {
		t.Fatal("status read changed durable state")
	}
	rpc(t, s, "operator", "messages.retry", map[string]any{"messageId": d.ID, "acknowledgeDuplicateRisk": true})
	if got := status(); got.Failed != 0 || got.Canceled != 0 || got.Ready != 1 {
		t.Fatal("retry retained stale failure classification")
	}
}

// Work stopped by abandonment must not be reported as waiting on a team, which
// would tell the operator a membership change could release it.
func TestRuntimeStatusSeparatesAbandonedWorkFromTeamWaiting(t *testing.T) {
	s, _, target := setup(t)
	joinWorkTeam(t, s, target, "product", "lead", "builder", "reviewer", "red")
	task := rpc(t, s, "lead", "tasks.assign", teamAssignment()).(Task)
	status := func() RuntimeQueueStatus {
		page := rpc(t, s, "operator", "runtime.status", map[string]any{"sessionId": "builder"}).(RuntimeStatusPage)
		if len(page.Sessions) != 1 {
			t.Fatal("expected exactly one identity")
		}
		return page.Sessions[0]
	}
	if got := status(); got.Ready != 1 || got.WaitingAbandoned != 0 {
		t.Fatalf("assigned work not ready: %+v", got)
	}
	rpc(t, s, "operator", "tasks.abandon", map[string]any{"id": task.ID, "evidence": "Experiment identity will never resubmit."})
	got := status()
	if got.WaitingAbandoned != 1 {
		t.Fatalf("abandoned work not counted as abandoned: %+v", got)
	}
	if got.WaitingTeam != 0 {
		t.Fatalf("abandoned work reported as waiting on a team: %+v", got)
	}
	if got.Ready != 0 || got.BlockedPolicy != 0 || got.ManualPending != 0 {
		t.Fatalf("abandoned work still counted as actionable: %+v", got)
	}
	// The team cause must still be reported under its own name.
	other := rpc(t, s, "lead", "tasks.assign", map[string]any{"teamId": "product", "to": "builder", "title": "Second", "text": "More work", "criteria": "Evidence", "reviewer": "reviewer", "redTeam": "red", "idempotencyKey": "second"}).(Task)
	rpc(t, s, "reviewer", "teams.leave", map[string]any{"id": "product"})
	if got := status(); got.WaitingTeam != 1 || got.WaitingAbandoned != 1 {
		t.Fatalf("the two causes did not stay separable: %+v (task %s)", got, other.ID)
	}
}

func TestRuntimeStatusPaginationCannotEscapeIdentityScope(t *testing.T) {
	s, _, _ := setup(t)
	seen := map[string]bool{}
	cursor := ""
	for {
		page := rpc(t, s, "operator", "runtime.status", map[string]any{"limit": 1, "cursor": cursor}).(RuntimeStatusPage)
		if len(page.Sessions) != 1 || seen[page.Sessions[0].Identity] {
			t.Fatal("invalid bounded page")
		}
		seen[page.Sessions[0].Identity] = true
		cursor = page.NextCursor
		if cursor == "" {
			break
		}
	}
	if len(seen) != 4 {
		t.Fatal("missing managed identities")
	}
	denied(t, s, "lead", "runtime.status", map[string]any{"cursor": "builder"})
	denied(t, s, "operator", "runtime.status", map[string]any{"limit": 101})
	denied(t, s, "operator", "runtime.status", map[string]any{"limit": -1})
}
