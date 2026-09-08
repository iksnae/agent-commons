// SPDX-License-Identifier: MPL-2.0

package core

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// abandonFixture builds the case abandonment exists for: a submitted result
// carrying a rejection at the current revision, which only the author can move.
func abandonFixture(t *testing.T) (*Service, string, Task) {
	t.Helper()
	s, dir, _ := setup(t)
	task := rpc(t, s, "lead", "tasks.assign", map[string]any{"to": "builder", "title": "Inspect", "text": "Work", "criteria": "Evidence", "reviewer": "reviewer", "idempotencyKey": "abandon"}).(Task)
	claimed := claim(t, s, "builder")
	if err := s.Finish(claimed.ID, "", "Result", nil); err != nil {
		t.Fatal(err)
	}
	rpc(t, s, "reviewer", "tasks.review", map[string]any{"id": task.ID, "verdict": "rejected", "evidence": "Not adequate", "expectedRevision": 1})
	return s, dir, rpc(t, s, "operator", "tasks.get", map[string]any{"id": task.ID}).(Task)
}

func abandonArgs(id string) map[string]any {
	return map[string]any{"id": id, "evidence": "Author identity is an abandoned experiment; it will never resubmit."}
}

func TestAbandonPreservesOutputAndReviews(t *testing.T) {
	s, _, before := abandonFixture(t)
	if before.Status != "submitted" || strings.TrimSpace(before.Output) == "" || len(before.Reviews) != 1 {
		t.Fatalf("fixture did not reach a rejected submission: %+v", before)
	}
	priorOutput := before.Output
	priorReviews, err := json.Marshal(before.Reviews)
	if err != nil {
		t.Fatal(err)
	}
	after := rpc(t, s, "operator", "tasks.abandon", abandonArgs(before.ID)).(Task)
	if after.Status != "abandoned" {
		t.Fatalf("status is %q, want abandoned", after.Status)
	}
	if after.Output != priorOutput {
		t.Fatalf("output changed: %q -> %q", priorOutput, after.Output)
	}
	afterReviews, err := json.Marshal(after.Reviews)
	if err != nil {
		t.Fatal(err)
	}
	if string(afterReviews) != string(priorReviews) {
		t.Fatalf("reviews changed: %s -> %s", priorReviews, afterReviews)
	}
	if after.Revision != before.Revision {
		t.Fatalf("revision changed: %d -> %d", before.Revision, after.Revision)
	}
}

func TestAbandonIsNotReversible(t *testing.T) {
	s, _, task := abandonFixture(t)
	rpc(t, s, "operator", "tasks.abandon", abandonArgs(task.ID))
	denied(t, s, "builder", "tasks.submit", map[string]any{"id": task.ID, "output": "Second attempt", "expectedRevision": 1})
	denied(t, s, "reviewer", "tasks.review", map[string]any{"id": task.ID, "verdict": "approved", "evidence": "Reconsidered", "expectedRevision": 1})
	denied(t, s, "lead", "tasks.accept", map[string]any{"id": task.ID})
	denied(t, s, "operator", "tasks.accept", map[string]any{"id": task.ID})
	denied(t, s, "operator", "tasks.abandon", map[string]any{"id": task.ID, "evidence": "Rewriting the recorded reason."})
	current := rpc(t, s, "operator", "tasks.get", map[string]any{"id": task.ID}).(Task)
	if current.Status != "abandoned" {
		t.Fatalf("status is %q, want abandoned", current.Status)
	}
}

// Abandonment must never become a way to pass a task.
func TestAbandonNeverSatisfiesAcceptance(t *testing.T) {
	s, _, task := abandonFixture(t)
	rpc(t, s, "operator", "tasks.abandon", abandonArgs(task.ID))
	for _, reviewer := range []string{"reviewer", "red", "lead", "operator"} {
		b, _ := json.Marshal(map[string]any{"id": task.ID, "verdict": "approved", "evidence": "Independent evidence", "expectedRevision": 1})
		_, _ = s.Call(reviewer, "tasks.review", b)
	}
	for _, actor := range []string{"lead", "operator"} {
		b, _ := json.Marshal(map[string]any{"id": task.ID})
		_, _ = s.Call(actor, "tasks.accept", b)
	}
	b, _ := json.Marshal(map[string]any{"id": task.ID, "output": "Resubmitted", "expectedRevision": 1})
	_, _ = s.Call("builder", "tasks.submit", b)
	if got := s.data.Tasks[task.ID].Status; got != "abandoned" {
		t.Fatalf("abandoned task reached %q", got)
	}
}

func TestAbandonEnqueuesNoDelivery(t *testing.T) {
	s, _, task := abandonFixture(t)
	before := len(s.data.Deliveries)
	rpc(t, s, "operator", "tasks.abandon", abandonArgs(task.ID))
	if after := len(s.data.Deliveries); after != before {
		t.Fatalf("delivery count %d -> %d; abandonment must enqueue nothing", before, after)
	}
}

func TestAbandonRefusedFromAccepted(t *testing.T) {
	s, _, task := abandonFixture(t)
	rpc(t, s, "builder", "tasks.submit", map[string]any{"id": task.ID, "output": "Corrected result", "expectedRevision": 1})
	rpc(t, s, "reviewer", "tasks.review", map[string]any{"id": task.ID, "verdict": "approved", "evidence": "Independent evidence", "expectedRevision": 2})
	accepted := rpc(t, s, "lead", "tasks.accept", map[string]any{"id": task.ID}).(Task)
	if accepted.Status != "accepted" {
		t.Fatalf("fixture did not reach accepted: %q", accepted.Status)
	}
	denied(t, s, "operator", "tasks.abandon", abandonArgs(task.ID))
	if got := s.data.Tasks[task.ID].Status; got != "accepted" {
		t.Fatalf("accepted task became %q", got)
	}
}

func TestAbandonIsOperatorOnly(t *testing.T) {
	s, _, task := abandonFixture(t)
	for _, actor := range []string{"lead", "builder", "reviewer", "red"} {
		denied(t, s, actor, "tasks.abandon", abandonArgs(task.ID))
	}
	if got := s.data.Tasks[task.ID].Status; got != "submitted" {
		t.Fatalf("non-operator abandonment moved status to %q", got)
	}
}

func TestAbandonRequiresBoundedEvidence(t *testing.T) {
	s, _, task := abandonFixture(t)
	denied(t, s, "operator", "tasks.abandon", map[string]any{"id": task.ID})
	denied(t, s, "operator", "tasks.abandon", map[string]any{"id": task.ID, "evidence": ""})
	denied(t, s, "operator", "tasks.abandon", map[string]any{"id": task.ID, "evidence": "   \t\n  "})
	denied(t, s, "operator", "tasks.abandon", map[string]any{"id": task.ID, "evidence": strings.Repeat("e", 8193)})
	if got := s.data.Tasks[task.ID].Status; got != "submitted" {
		t.Fatalf("refused evidence still moved status to %q", got)
	}
	rpc(t, s, "operator", "tasks.abandon", map[string]any{"id": task.ID, "evidence": strings.Repeat("e", 8192)})
}

func TestAbandonSurvivesRestart(t *testing.T) {
	s, dir, task := abandonFixture(t)
	rpc(t, s, "operator", "tasks.abandon", abandonArgs(task.ID))
	before := s.data.Tasks[task.ID]
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	after := reopened.data.Tasks[task.ID]
	if after.Status != "abandoned" {
		t.Fatalf("status after restart is %q, want abandoned", after.Status)
	}
	if after.Output != before.Output || len(after.Reviews) != len(before.Reviews) {
		t.Fatalf("restart altered retained evidence: %+v -> %+v", before, after)
	}
}

func TestAbandonRollsBackWhenSaveFails(t *testing.T) {
	s, _, task := abandonFixture(t)
	before, err := json.Marshal(s.data.Tasks[task.ID])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(s.dir, 0o500); err != nil {
		t.Fatal(err)
	}
	denied(t, s, "operator", "tasks.abandon", abandonArgs(task.ID))
	if err := os.Chmod(s.dir, 0o700); err != nil {
		t.Fatal(err)
	}
	after, err := json.Marshal(s.data.Tasks[task.ID])
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("failed save left a half-applied abandonment: %s -> %s", before, after)
	}
	// The rolled-back service must still be able to abandon once saving works.
	rpc(t, s, "operator", "tasks.abandon", abandonArgs(task.ID))
}

// Abandonment is the escape that lets a stuck identity be restricted again.
func TestAbandonReleasesTheUnresolvedTaskPolicyRestriction(t *testing.T) {
	s, _, task := abandonFixture(t)
	denied(t, s, "operator", "sessions.policy", map[string]any{"id": "builder", "policy": "coordination"})
	denied(t, s, "operator", "sessions.policy", map[string]any{"id": "reviewer", "policy": "coordination"})
	rpc(t, s, "operator", "tasks.abandon", abandonArgs(task.ID))
	rpc(t, s, "operator", "sessions.policy", map[string]any{"id": "builder", "policy": "coordination"})
	rpc(t, s, "operator", "sessions.policy", map[string]any{"id": "reviewer", "policy": "coordination"})
}

// A queued or retryable task delivery must not resurrect an abandoned task.
func TestAbandonStopsQueuedAndRetryableTaskWork(t *testing.T) {
	s, _, _ := setup(t)
	task := rpc(t, s, "lead", "tasks.assign", map[string]any{"to": "builder", "title": "Inspect", "text": "Work", "criteria": "Evidence", "reviewer": "reviewer", "idempotencyKey": "queued"}).(Task)
	rpc(t, s, "operator", "tasks.abandon", abandonArgs(task.ID))
	if d, err := s.Claim("builder"); err != nil || d != nil {
		t.Fatalf("abandoned task work claimed: %v %+v", err, d)
	}
	if got := s.data.Tasks[task.ID].Status; got != "abandoned" {
		t.Fatalf("claim moved abandoned task to %q", got)
	}
	var deliveryID string
	for i := range s.data.Deliveries {
		if s.data.Deliveries[i].TaskID == task.ID && s.data.Deliveries[i].Kind == "task" {
			deliveryID = s.data.Deliveries[i].ID
			s.data.Deliveries[i].Status = "failed"
		}
	}
	if deliveryID == "" {
		t.Fatal("no task delivery to retry")
	}
	denied(t, s, "operator", "messages.retry", map[string]any{"messageId": deliveryID, "acknowledgeDuplicateRisk": true})
	if s.RuntimeActive(deliveryID) {
		t.Fatal("abandoned task delivery reported active")
	}
}

// Abandonment is the operator coordination the team-participant gate demands,
// so it must not itself be blocked by that gate.
func TestAbandonWorksWhenTeamParticipantLeft(t *testing.T) {
	s, _, target := setup(t)
	joinWorkTeam(t, s, target, "product", "lead", "builder", "reviewer", "red")
	task := rpc(t, s, "lead", "tasks.assign", teamAssignment()).(Task)
	rpc(t, s, "operator", "teams.revoke", map[string]any{"id": "product", "target": target, "to": "builder"})
	denied(t, s, "builder", "tasks.submit", map[string]any{"id": task.ID, "output": "Revoked", "expectedRevision": 0})
	rpc(t, s, "operator", "tasks.abandon", abandonArgs(task.ID))
	if got := s.data.Tasks[task.ID].Status; got != "abandoned" {
		t.Fatalf("status is %q, want abandoned", got)
	}
}
