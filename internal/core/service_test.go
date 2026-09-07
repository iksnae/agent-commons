// SPDX-License-Identifier: MPL-2.0

package core

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func rpc(t *testing.T, s *Service, a, m string, p any) any {
	t.Helper()
	b, e := json.Marshal(p)
	if e != nil {
		t.Fatal(e)
	}
	v, e := s.Call(a, m, b)
	if e != nil {
		t.Fatalf("%s by %s: %v", m, a, e)
	}
	return v
}
func denied(t *testing.T, s *Service, a, m string, p any) {
	t.Helper()
	b, _ := json.Marshal(p)
	if _, e := s.Call(a, m, b); e == nil {
		t.Fatalf("expected denial: %s by %s", m, a)
	}
}
func setup(t *testing.T) (*Service, string, string) {
	t.Helper()
	dir := t.TempDir()
	target := t.TempDir()
	s, e := New(dir)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	for _, id := range []string{"lead", "builder", "reviewer", "red"} {
		rpc(t, s, "operator", "sessions.register", Session{ID: id, Target: target, Mode: "managed", Runtime: "codex", Policy: "workflow"})
	}
	return s, dir, target
}
func send(t *testing.T, s *Service, from, to, key string) Delivery {
	return rpc(t, s, from, "messages.send", map[string]any{"to": to, "text": "hello", "idempotencyKey": key}).(Delivery)
}
func claim(t *testing.T, s *Service, id string) *Delivery {
	t.Helper()
	d, e := s.Claim(id)
	if e != nil || d == nil {
		t.Fatalf("claim %s: %v %+v", id, e, d)
	}
	return d
}

func TestDeliveryRestartAndReturnPath(t *testing.T) {
	s, dir, _ := setup(t)
	d := send(t, s, "lead", "builder", "one")
	dup := send(t, s, "lead", "builder", "one")
	if dup.ID != d.ID {
		t.Fatal("duplicate not suppressed")
	}
	tok, _ := s.Token("lead")
	if _, e := New(dir); e == nil {
		t.Fatal("second process lock allowed")
	}
	s.Close()
	var e error
	s, e = New(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if a, ok := s.Authenticate(tok); !ok || a != "lead" {
		t.Fatal("credential not durable")
	}
	c := claim(t, s, "builder")
	if c.ID != d.ID || c.Attempts != 1 {
		t.Fatal(c)
	}
	s.Close()
	s, e = New(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	in := rpc(t, s, "builder", "inbox.list", nil).([]Delivery)
	if in[0].Status != "interrupted" {
		t.Fatal(in)
	}
	if d, e := s.Claim("builder"); d != nil || e != nil {
		t.Fatal("replayed uncertain work")
	}
	denied(t, s, "operator", "messages.retry", map[string]any{"messageId": c.ID})
	rpc(t, s, "operator", "messages.retry", map[string]any{"messageId": c.ID, "acknowledgeDuplicateRisk": true})
	c = claim(t, s, "builder")
	if c.Attempts != 2 {
		t.Fatal(c)
	}
	if e = s.Finish(c.ID, "runtime-1", "answer", nil); e != nil {
		t.Fatal(e)
	}
	if e = s.Finish(c.ID, "runtime-1", "answer", nil); e == nil {
		t.Fatal("double finish")
	}
	r := claim(t, s, "lead")
	if r.Kind != "result" || r.From != "builder" || r.Text != "answer" {
		t.Fatal(r)
	}
	if e = s.Finish(r.ID, "lead-runtime", "ack", nil); e != nil {
		t.Fatal(e)
	}
	in = rpc(t, s, "builder", "inbox.list", nil).([]Delivery)
	if len(in) != 1 {
		t.Fatal("result auto-replied", in)
	}
	if s.Sessions()[0].RuntimeSessionID != "runtime-1" {
		t.Fatal(s.Sessions())
	}
}

func TestIsolationCredentialsAndValidation(t *testing.T) {
	s, _, target := setup(t)
	other := t.TempDir()
	rpc(t, s, "operator", "sessions.register", Session{ID: "outsider", Target: other, Runtime: "manual", Mode: "manual"})
	denied(t, s, "builder", "sessions.register", Session{ID: "x", Target: target, Runtime: "manual", Mode: "manual"})
	denied(t, s, "operator", "sessions.register", Session{ID: "../bad", Target: target, Runtime: "manual", Mode: "manual"})
	denied(t, s, "operator", "sessions.register", Session{ID: "x", Target: "relative", Runtime: "manual", Mode: "manual"})
	denied(t, s, "operator", "sessions.register", Session{ID: "x", Target: target, Runtime: "codex", Mode: "managed", RuntimeSessionID: "adopted"})
	denied(t, s, "unknown", "sessions.list", nil)
	denied(t, s, "outsider", "messages.send", map[string]any{"to": "builder", "text": "hi", "idempotencyKey": "x"})
	d := send(t, s, "lead", "builder", "x")
	denied(t, s, "lead", "inbox.acknowledge", map[string]any{"messageId": d.ID})
	denied(t, s, "lead", "inbox.list", map[string]any{"sessionId": "builder"})
	rpc(t, s, "builder", "inbox.acknowledge", map[string]any{"messageId": d.ID})
	if len(rpc(t, s, "outsider", "sessions.list", nil).([]Session)) != 1 {
		t.Fatal("cross target discovery")
	}
	token, _ := s.Token("builder")
	if _, ok := s.Authenticate(""); ok {
		t.Fatal("empty token authenticated")
	}
	b, _ := json.Marshal(rpc(t, s, "operator", "sessions.list", nil))
	if contains(string(b), token) {
		t.Fatal("token leaked")
	}
	sessions := s.Sessions()
	sessions[0].Target = "corrupt"
	if s.Sessions()[0].Target == "corrupt" {
		t.Fatal("mutable snapshot")
	}
}
func contains(s, v string) bool {
	for i := 0; i+len(v) <= len(s); i++ {
		if s[i:i+len(v)] == v {
			return true
		}
	}
	return false
}

func TestContextVersionsAndPinnedDelivery(t *testing.T) {
	s, _, target := setup(t)
	rpc(t, s, "lead", "context.put", map[string]any{"id": "brief", "text": "original", "expectedVersion": 0})
	d := rpc(t, s, "lead", "messages.send", map[string]any{"to": "builder", "text": "task", "idempotencyKey": "pinned", "contextId": "brief"}).(Delivery)
	rpc(t, s, "lead", "context.put", map[string]any{"id": "brief", "text": "new", "expectedVersion": 1})
	denied(t, s, "lead", "context.put", map[string]any{"id": "brief", "text": "bad", "expectedVersion": 1})
	old := rpc(t, s, "builder", "context.get", map[string]any{"id": "brief", "version": 1}).(Context)
	if old.Text != "original" || d.ContextVersion != 1 || !contains(d.Text, "original") {
		t.Fatal(old, d)
	}
	denied(t, s, "builder", "context.get", map[string]any{"id": "brief", "target": t.TempDir()})
	rpc(t, s, "operator", "context.get", map[string]any{"id": "brief", "target": target})
	denied(t, s, "builder", "messages.send", map[string]any{"to": "lead", "text": "x", "idempotencyKey": "bad", "contextId": "brief", "contextVersion": 3})
}

func TestIndependentReviewGates(t *testing.T) {
	s, _, _ := setup(t)
	p := map[string]any{"to": "builder", "title": "Build", "text": "Implement", "criteria": "evidence", "reviewer": "reviewer", "redTeam": "red", "idempotencyKey": "task"}
	task := rpc(t, s, "lead", "tasks.assign", p).(Task)
	dup := rpc(t, s, "lead", "tasks.assign", p).(Task)
	if task.ID != dup.ID {
		t.Fatal("task duplicated")
	}
	review := map[string]any{"id": task.ID, "verdict": "approved", "evidence": "inspected actual result", "expectedRevision": 1}
	denied(t, s, "reviewer", "tasks.review", review)
	c := claim(t, s, "builder")
	if e := s.Finish(c.ID, "run", "revision abc checks pass", nil); e != nil {
		t.Fatal(e)
	}
	denied(t, s, "lead", "tasks.accept", map[string]any{"id": task.ID})
	denied(t, s, "builder", "tasks.review", review)
	denied(t, s, "lead", "tasks.review", review)
	rpc(t, s, "reviewer", "tasks.review", review)
	denied(t, s, "reviewer", "tasks.review", review)
	denied(t, s, "lead", "tasks.accept", map[string]any{"id": task.ID})
	rpc(t, s, "red", "tasks.review", review)
	denied(t, s, "reviewer", "tasks.accept", map[string]any{"id": task.ID})
	done := rpc(t, s, "lead", "tasks.accept", map[string]any{"id": task.ID}).(Task)
	if done.Status != "accepted" || done.Revision != 1 {
		t.Fatal(done)
	}
	done.Reviews[0].Verdict = "rejected"
	stored := rpc(t, s, "lead", "tasks.get", map[string]any{"id": task.ID}).(Task)
	if stored.Reviews[0].Verdict != "approved" {
		t.Fatal("mutable reviews")
	}
	p["idempotencyKey"] = "self"
	p["reviewer"] = "builder"
	denied(t, s, "lead", "tasks.assign", p)
	p["reviewer"] = "reviewer"
	p["redTeam"] = "reviewer"
	denied(t, s, "lead", "tasks.assign", p)
}

func TestFailureAndConcurrentClaim(t *testing.T) {
	s, _, _ := setup(t)
	send(t, s, "lead", "builder", "1")
	send(t, s, "lead", "builder", "2")
	var wg sync.WaitGroup
	results := make(chan *Delivery, 20)
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d, e := s.Claim("builder")
			if e != nil {
				t.Error(e)
			}
			if d != nil {
				results <- d
			}
		}()
	}
	wg.Wait()
	close(results)
	var claimed []*Delivery
	for d := range results {
		claimed = append(claimed, d)
	}
	if len(claimed) != 1 {
		t.Fatal("concurrent runs", len(claimed))
	}
	if e := s.Finish(claimed[0].ID, "", "", errors.New("runtime unavailable")); e != nil {
		t.Fatal(e)
	}
	if len(rpc(t, s, "lead", "inbox.list", nil).([]Delivery)) != 0 {
		t.Fatal("failure masqueraded as result")
	}
	claim(t, s, "builder")
}

func TestWriteFailureRollsBack(t *testing.T) {
	s, dir, target := setup(t)
	if e := os.Rename(dir, dir+"-moved"); e != nil {
		t.Fatal(e)
	}
	defer os.RemoveAll(dir + "-moved")
	denied(t, s, "operator", "sessions.register", Session{ID: "lost", Target: target, Runtime: "manual", Mode: "manual"})
	if _, e := s.Token("lost"); e == nil {
		t.Fatal("failed write mutated memory")
	}
	if e := os.Rename(dir+"-moved", dir); e != nil {
		t.Fatal(e)
	}
	info, e := os.Stat(filepath.Join(dir, "state.json"))
	if e != nil || info.Mode().Perm() != 0600 {
		t.Fatal("state permissions", e)
	}
}

func TestRejectedTaskAndEmptyOutput(t *testing.T) {
	s, _, _ := setup(t)
	p := map[string]any{"to": "builder", "title": "Review", "text": "work", "criteria": "actual evidence", "reviewer": "reviewer", "idempotencyKey": "reject"}
	task := rpc(t, s, "lead", "tasks.assign", p).(Task)
	d := claim(t, s, "builder")
	if err := s.Finish(d.ID, "session", "", nil); err != nil {
		t.Fatal(err)
	}
	got := rpc(t, s, "lead", "tasks.get", map[string]any{"id": task.ID}).(Task)
	if got.Status != "failed" {
		t.Fatal(got)
	}
	rpc(t, s, "operator", "messages.retry", map[string]any{"messageId": d.ID, "acknowledgeDuplicateRisk": true})
	d = claim(t, s, "builder")
	if err := s.Finish(d.ID, "session", "candidate", nil); err != nil {
		t.Fatal(err)
	}
	rpc(t, s, "reviewer", "tasks.review", map[string]any{"id": task.ID, "verdict": "rejected", "evidence": "missing required behavior", "expectedRevision": 1})
	denied(t, s, "reviewer", "tasks.review", map[string]any{"id": task.ID, "verdict": "approved", "evidence": "changed my mind"})
	denied(t, s, "operator", "tasks.accept", map[string]any{"id": task.ID})
	rpc(t, s, "operator", "sessions.register", Session{ID: "outsider", Target: t.TempDir(), Runtime: "manual", Mode: "manual"})
	denied(t, s, "outsider", "tasks.get", map[string]any{"id": task.ID})
	if len(rpc(t, s, "outsider", "tasks.list", nil).([]Task)) != 0 {
		t.Fatal("task isolation failed")
	}
}

func TestManualSubmissionRevisionAndManagedAcknowledgement(t *testing.T) {
	s, _, _ := setup(t)
	task := rpc(t, s, "lead", "tasks.assign", map[string]any{"to": "builder", "title": "revision", "text": "work", "criteria": "prove", "reviewer": "reviewer", "idempotencyKey": "revision"}).(Task)
	in := rpc(t, s, "builder", "inbox.list", nil).([]Delivery)
	ack := rpc(t, s, "builder", "inbox.acknowledge", map[string]any{"messageId": in[0].ID}).(Delivery)
	if !ack.Acknowledged || ack.Status != "pending" {
		t.Fatal(ack)
	}
	d := claim(t, s, "builder")
	if err := s.Finish(d.ID, "", "", errors.New("failed")); err != nil {
		t.Fatal(err)
	}
	denied(t, s, "lead", "tasks.submit", map[string]any{"id": task.ID, "output": "forged", "expectedRevision": 0})
	rpc(t, s, "builder", "tasks.submit", map[string]any{"id": task.ID, "output": "first", "expectedRevision": 0})
	rpc(t, s, "reviewer", "tasks.review", map[string]any{"id": task.ID, "verdict": "approved", "evidence": "first verified", "expectedRevision": 1})
	rpc(t, s, "builder", "tasks.submit", map[string]any{"id": task.ID, "output": "second", "expectedRevision": 1})
	denied(t, s, "builder", "tasks.submit", map[string]any{"id": task.ID, "output": "duplicate", "expectedRevision": 1})
	denied(t, s, "lead", "tasks.accept", map[string]any{"id": task.ID})
	denied(t, s, "reviewer", "tasks.review", map[string]any{"id": task.ID, "verdict": "approved", "evidence": "stale", "expectedRevision": 1})
	rpc(t, s, "reviewer", "tasks.review", map[string]any{"id": task.ID, "verdict": "approved", "evidence": "second verified", "expectedRevision": 2})
	rpc(t, s, "lead", "tasks.accept", map[string]any{"id": task.ID})
	denied(t, s, "operator", "messages.retry", map[string]any{"messageId": d.ID, "acknowledgeDuplicateRisk": true})
	denied(t, s, "builder", "tasks.submit", map[string]any{"id": task.ID, "output": "third", "expectedRevision": 2})
}

func TestResultRestartPreservesTaskAndExplicitSubmissionDoesNotDuplicate(t *testing.T) {
	s, dir, _ := setup(t)
	task := rpc(t, s, "lead", "tasks.assign", map[string]any{"to": "builder", "title": "one", "text": "work", "criteria": "prove", "reviewer": "reviewer", "idempotencyKey": "one"}).(Task)
	d := claim(t, s, "builder")
	rpc(t, s, "builder", "tasks.submit", map[string]any{"id": task.ID, "output": "explicit result", "expectedRevision": 0})
	if err := s.Finish(d.ID, "session", "final chatter", nil); err != nil {
		t.Fatal(err)
	}
	in := rpc(t, s, "lead", "inbox.list", nil).([]Delivery)
	if len(in) != 1 {
		t.Fatal("duplicate result", in)
	}
	claim(t, s, "lead")
	s.Close()
	var err error
	s, err = New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got := rpc(t, s, "lead", "tasks.get", map[string]any{"id": task.ID}).(Task)
	if got.Status != "submitted" || got.Output != "explicit result" || got.Revision != 1 {
		t.Fatal(got)
	}
}

func TestRejectSymlinkStateAndForgedSender(t *testing.T) {
	s, _, _ := setup(t)
	denied(t, s, "lead", "messages.send", map[string]any{"to": "builder", "text": "bad", "idempotencyKey": "bad", "from": "operator"})
	for _, name := range []string{"state.json", "lock"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			victim := filepath.Join(t.TempDir(), "victim")
			if err := os.WriteFile(victim, []byte("untouched"), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(victim, filepath.Join(dir, name)); err != nil {
				t.Fatal(err)
			}
			if srv, err := New(dir); err == nil {
				srv.Close()
				t.Fatal("symlink accepted")
			}
			b, _ := os.ReadFile(victim)
			if string(b) != "untouched" {
				t.Fatal("victim modified")
			}
		})
	}
}
