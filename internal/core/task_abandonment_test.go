// SPDX-License-Identifier: MPL-2.0

package core

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	// The refusal must describe the condition it actually guards: the task is
	// terminal, not merely unaccepted.
	b, _ := json.Marshal(map[string]any{"id": task.ID, "output": "Second attempt", "expectedRevision": 1})
	_, submitErr := s.Call("builder", "tasks.submit", b)
	if submitErr == nil {
		t.Fatal("abandoned task accepted a resubmission")
	}
	if !strings.Contains(submitErr.Error(), "non-terminal task") {
		t.Fatalf("submit refusal misdescribes its own condition: %q", submitErr)
	}
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
	s, _, first := abandonFixture(t)
	second := rpc(t, s, "lead", "tasks.assign", map[string]any{"to": "builder", "title": "Second", "text": "More work", "criteria": "Evidence", "reviewer": "reviewer", "idempotencyKey": "rollback"}).(Task)
	// Abandon once so the schema is already 4: the failure under test is then
	// the state write itself, not the pre-migration snapshot.
	rpc(t, s, "operator", "tasks.abandon", abandonArgs(first.ID))
	if s.data.SchemaVersion != 4 {
		t.Fatalf("schema is %d; the save path is not the one being tested", s.data.SchemaVersion)
	}
	before, err := json.Marshal(s.data)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(s.dir, 0o500); err != nil {
		t.Fatal(err)
	}
	denied(t, s, "operator", "tasks.abandon", abandonArgs(second.ID))
	if err := os.Chmod(s.dir, 0o700); err != nil {
		t.Fatal(err)
	}
	after, err := json.Marshal(s.data)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("failed save left a half-applied abandonment: %s -> %s", before, after)
	}
	// The rolled-back service must still be able to abandon once saving works.
	rpc(t, s, "operator", "tasks.abandon", abandonArgs(second.ID))
}

// backupSchema now serves two features, so its failure must name the migration
// that asked for it rather than the team migration it was written for.
func TestAbandonBackupFailureNamesAbandonmentNotTeams(t *testing.T) {
	s, _, task := abandonFixture(t)
	before := s.data.SchemaVersion
	if err := os.Chmod(s.dir, 0o500); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(abandonArgs(task.ID))
	_, err := s.Call("operator", "tasks.abandon", b)
	if chmodErr := os.Chmod(s.dir, 0o700); chmodErr != nil {
		t.Fatal(chmodErr)
	}
	if err == nil {
		t.Fatal("abandonment succeeded with an unwritable state directory")
	}
	if !strings.Contains(err.Error(), "pre-task-abandonment-schema backup failed") {
		t.Fatalf("backup failure does not name abandonment as the migration that asked: %q", err)
	}
	if strings.Contains(err.Error(), "team") {
		t.Fatalf("backup failure blames the team migration: %q", err)
	}
	if s.data.SchemaVersion != before {
		t.Fatalf("failed backup still advanced the schema to %d", s.data.SchemaVersion)
	}
	if got := s.data.Tasks[task.ID].Status; got != "submitted" {
		t.Fatalf("failed backup still moved the task to %q", got)
	}
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

// Abandoning a task while its turn is in flight must discard the result rather
// than record it, and must say abandonment is why.
func TestAbandonDuringRunWithholdsTheFinishedResult(t *testing.T) {
	s, _, _ := setup(t)
	task := rpc(t, s, "lead", "tasks.assign", map[string]any{"to": "builder", "title": "Inspect", "text": "Work", "criteria": "Evidence", "reviewer": "reviewer", "idempotencyKey": "inflight"}).(Task)
	d := claim(t, s, "builder")
	if s.data.Tasks[task.ID].Status != "working" {
		t.Fatal("claim did not start the task")
	}
	rpc(t, s, "operator", "tasks.abandon", abandonArgs(task.ID))
	if err := s.Finish(d.ID, "", "Result the operator no longer wants", nil); err != nil {
		t.Fatal(err)
	}
	if got := s.data.Tasks[task.ID].Status; got != "abandoned" {
		t.Fatalf("finish moved the abandoned task to %q", got)
	}
	var finished Delivery
	for _, candidate := range s.data.Deliveries {
		if candidate.ID == d.ID {
			finished = candidate
		}
	}
	if finished.Status != "failed" {
		t.Fatalf("delivery status is %q, want failed", finished.Status)
	}
	if finished.Output != "" {
		t.Fatalf("result recorded against an abandoned task: %q", finished.Output)
	}
	if !strings.Contains(finished.Error, "abandoned") {
		t.Fatalf("withholding reason does not name abandonment: %q", finished.Error)
	}
	if s.data.Sessions["builder"].Busy {
		t.Fatal("session left busy after finishing abandoned work")
	}
}

// The result delivery reporting an abandoned task must stop costing runtime
// turns without becoming unreadable.
func TestAbandonStopsResultDeliveryButKeepsItReadable(t *testing.T) {
	s, _, _ := setup(t)
	rpc(t, s, "operator", "sessions.register", Session{ID: "manager", Target: s.data.Sessions["lead"].Target, Mode: "managed", Runtime: "codex", Policy: "workflow"})
	task := rpc(t, s, "manager", "tasks.assign", map[string]any{"to": "builder", "title": "Inspect", "text": "Work", "criteria": "Evidence", "reviewer": "reviewer", "idempotencyKey": "result"}).(Task)
	d := claim(t, s, "builder")
	if err := s.Finish(d.ID, "", "Delivered result", nil); err != nil {
		t.Fatal(err)
	}
	var result Delivery
	for _, candidate := range s.data.Deliveries {
		if candidate.To == "manager" && candidate.Kind == "result" {
			result = candidate
		}
	}
	if result.ID == "" || result.TaskID != task.ID || result.Status != "pending" {
		t.Fatalf("fixture lacks a pending result delivery for the task: %+v", result)
	}
	if claimable, err := s.Claim("manager"); err != nil || claimable == nil {
		t.Fatalf("result was not claimable before abandonment: %v %+v", err, claimable)
	}
	s.data.Deliveries[len(s.data.Deliveries)-1].Status = "pending"
	manager := s.data.Sessions["manager"]
	manager.Busy = false
	s.data.Sessions["manager"] = manager

	rpc(t, s, "operator", "tasks.abandon", abandonArgs(task.ID))
	if claimable, err := s.Claim("manager"); err != nil || claimable != nil {
		t.Fatalf("managed turn spent on a terminated task: %v %+v", err, claimable)
	}
	// Refusing the turn must not hide the result from the lead who is owed it.
	found := false
	for _, listed := range rpc(t, s, "manager", "inbox.list", nil).([]Delivery) {
		if listed.ID == result.ID {
			found = true
			// Finish carries the payload in Text; a result delivery's Output
			// is empty, so asserting on it would prove nothing.
			if listed.Text != result.Text {
				t.Fatalf("abandonment rewrote peer result content: %q -> %q", result.Text, listed.Text)
			}
			if !strings.Contains(listed.Text, "Delivered result") {
				t.Fatalf("result text lost the author's payload: %q", listed.Text)
			}
		}
	}
	if !found {
		t.Fatal("result delivery vanished from the inbox it was addressed to")
	}
	if got := s.runtimeQueue(s.data.Sessions["manager"]); got.WaitingAbandoned != 1 || got.WaitingTeam != 0 || got.Ready != 0 {
		t.Fatalf("abandoned work miscounted: %+v", got)
	}
	for i := range s.data.Deliveries {
		if s.data.Deliveries[i].ID == result.ID {
			s.data.Deliveries[i].Status = "failed"
		}
	}
	denied(t, s, "operator", "messages.retry", map[string]any{"messageId": result.ID, "acknowledgeDuplicateRisk": true})
}

// Abandoning while a result delivery is in flight is not the no-op the pending
// case is: Finish discards that turn's output and retry stays refused. Only the
// text the recipient was already sent survives.
func TestAbandonDuringResultTurnDiscardsThatTurnsOutput(t *testing.T) {
	s, _, target := setup(t)
	rpc(t, s, "operator", "sessions.register", Session{ID: "manager", Target: target, Mode: "managed", Runtime: "codex", Policy: "workflow"})
	task := rpc(t, s, "manager", "tasks.assign", map[string]any{"to": "builder", "title": "Inspect", "text": "Work", "criteria": "Evidence", "reviewer": "reviewer", "idempotencyKey": "inflight-result"}).(Task)
	assigned := claim(t, s, "builder")
	if err := s.Finish(assigned.ID, "", "Delivered result", nil); err != nil {
		t.Fatal(err)
	}
	result := claim(t, s, "manager")
	if result.Kind != "result" || result.TaskID != task.ID {
		t.Fatalf("did not claim the result delivery: %+v", result)
	}
	sentText := result.Text
	rpc(t, s, "operator", "tasks.abandon", abandonArgs(task.ID))
	if err := s.Finish(result.ID, "", "Lead's reaction to a terminated task", nil); err != nil {
		t.Fatal(err)
	}
	var finished Delivery
	for _, candidate := range s.data.Deliveries {
		if candidate.ID == result.ID {
			finished = candidate
		}
	}
	if finished.Status != "failed" {
		t.Fatalf("in-flight result delivery status is %q, want failed", finished.Status)
	}
	if finished.Output != "" {
		t.Fatalf("output recorded for a turn on a terminated task: %q", finished.Output)
	}
	if !strings.Contains(finished.Error, "abandoned") {
		t.Fatalf("withholding reason does not name abandonment: %q", finished.Error)
	}
	if finished.Text != sentText {
		t.Fatalf("the text the recipient was sent was rewritten: %q -> %q", sentText, finished.Text)
	}
	// Failed plus abandoned is exactly the state messages.retry must refuse.
	denied(t, s, "operator", "messages.retry", map[string]any{"messageId": result.ID, "acknowledgeDuplicateRisk": true})
}

// A directory that abandons a task must stop being readable by a binary whose
// terminality check would let that task be accepted.
func TestAbandonAdvancesSchemaLazilyAndOnlyOnSuccess(t *testing.T) {
	s, dir, task := abandonFixture(t)
	if s.data.SchemaVersion != 1 {
		t.Fatalf("fixture schema is %d; adjust this test to its real starting point", s.data.SchemaVersion)
	}
	before := s.data.SchemaVersion
	denied(t, s, "lead", "tasks.abandon", abandonArgs(task.ID))
	denied(t, s, "operator", "tasks.abandon", map[string]any{"id": task.ID, "evidence": " "})
	denied(t, s, "operator", "tasks.abandon", abandonArgs("no-such-task"))
	if s.data.SchemaVersion != before {
		t.Fatalf("refused abandonment advanced the schema to %d", s.data.SchemaVersion)
	}
	if backups := schemaBackups(t, dir); len(backups) != 0 {
		t.Fatalf("refused abandonment wrote backups: %v", backups)
	}
	rpc(t, s, "operator", "tasks.abandon", abandonArgs(task.ID))
	if s.data.SchemaVersion != 4 {
		t.Fatalf("schema is %d after abandonment, want 4", s.data.SchemaVersion)
	}
	if backups := schemaBackups(t, dir); len(backups) != 1 {
		t.Fatalf("want exactly one pre-abandonment snapshot, got %v", backups)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if reopened.data.SchemaVersion != 4 {
		t.Fatalf("schema did not survive restart: %d", reopened.data.SchemaVersion)
	}
	// A newer schema than this binary knows must be refused, not read.
	reopened.data.SchemaVersion = 5
	if err := reopened.save(); err != nil {
		t.Fatal(err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
	if refused, err := New(dir); err == nil {
		refused.Close()
		t.Fatal("binary read a state schema newer than itself")
	}
}

func schemaBackups(t *testing.T, dir string) []string {
	t.Helper()
	names, err := filepath.Glob(filepath.Join(dir, "pre-task-abandonment-schema-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	return names
}

// A directory that never abandons anything stays on the older schema.
func TestUnusedAbandonmentLeavesSchemaAlone(t *testing.T) {
	s, _, task := abandonFixture(t)
	before := s.data.SchemaVersion
	rpc(t, s, "builder", "tasks.submit", map[string]any{"id": task.ID, "output": "Corrected", "expectedRevision": 1})
	rpc(t, s, "reviewer", "tasks.review", map[string]any{"id": task.ID, "verdict": "approved", "evidence": "Independent evidence", "expectedRevision": 2})
	rpc(t, s, "lead", "tasks.accept", map[string]any{"id": task.ID})
	if s.data.SchemaVersion != before {
		t.Fatalf("schema advanced to %d without any abandonment", s.data.SchemaVersion)
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
