package core

import (
	"strings"
	"testing"
)

func TestPolicyProvenanceAndCorrelation(t *testing.T) {
	s, dir, target := setup(t)
	rpc(t, s, "operator", "sessions.register", Session{ID: "manual", Target: target, Runtime: "claude", Mode: "manual"})
	denied(t, s, "manual", "context.put", map[string]any{"id": "x", "text": "x"})
	denied(t, s, "manual", "tasks.assign", map[string]any{})
	denied(t, s, "manual", "sessions.policy", map[string]any{"id": "manual", "policy": "workflow"})
	denied(t, s, "manual", "messages.send", map[string]any{"to": "lead", "text": "x", "idempotencyKey": "bad", "provenance": "operator-direct"})
	d := send(t, s, "manual", "lead", "hello")
	if d.Provenance != "peer-assertion" || d.GrantsAuthority || d.ThreadID != d.ID {
		t.Fatal(d)
	}
	r := rpc(t, s, "lead", "messages.send", map[string]any{"to": "manual", "text": "reply", "idempotencyKey": "reply", "replyTo": d.ID, "provenance": "peer-relayed"}).(Delivery)
	if r.ReplyTo != d.ID || r.ThreadID != d.ThreadID || r.Provenance != "peer-relayed" {
		t.Fatal(r)
	}
	denied(t, s, "reviewer", "messages.send", map[string]any{"to": "manual", "text": "forged reply", "idempotencyKey": "badreply", "replyTo": d.ID})
	s.Close()
	reopened, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	denied(t, reopened, "manual", "context.put", map[string]any{"id": "x", "text": "x"})
}

func TestInboxPagesAndHandling(t *testing.T) {
	s, _, _ := setup(t)
	a := send(t, s, "lead", "builder", "a")
	b := send(t, s, "lead", "builder", "b")
	send(t, s, "lead", "builder", "c")
	p := rpc(t, s, "builder", "inbox.page", map[string]any{"limit": 1, "unreadOnly": true}).(map[string]any)
	if p["nextCursor"] != a.ID || len(p["messages"].([]Delivery)) != 1 {
		t.Fatal(p)
	}
	denied(t, s, "builder", "inbox.handle", map[string]any{"messageId": a.ID, "evidence": "done"})
	rpc(t, s, "builder", "inbox.acknowledge", map[string]any{"messageId": a.ID})
	h := rpc(t, s, "builder", "inbox.handle", map[string]any{"messageId": a.ID, "evidence": "triaged"}).(Delivery)
	if !h.Handled || h.Status != "pending" {
		t.Fatal(h)
	}
	denied(t, s, "builder", "inbox.handle", map[string]any{"messageId": a.ID, "evidence": "changed"})
	denied(t, s, "lead", "inbox.handle", map[string]any{"messageId": a.ID, "evidence": "triaged"})
	p = rpc(t, s, "builder", "inbox.page", map[string]any{"limit": 1, "cursor": a.ID, "unreadOnly": true}).(map[string]any)
	if p["messages"].([]Delivery)[0].ID != b.ID {
		t.Fatal(p)
	}
	denied(t, s, "reviewer", "inbox.page", map[string]any{"cursor": a.ID})
	denied(t, s, "builder", "inbox.page", map[string]any{"limit": 101})
	p = rpc(t, s, "builder", "inbox.page", map[string]any{"unhandledOnly": true}).(map[string]any)
	if len(p["messages"].([]Delivery)) != 2 {
		t.Fatal(p)
	}
}

func TestAutomaticReplyCorrelationAndOutputBounds(t *testing.T) {
	s, _, _ := setup(t)
	d := send(t, s, "lead", "builder", "auto")
	claim(t, s, "builder")
	if err := s.Finish(d.ID, "native", "ok", nil); err != nil {
		t.Fatal(err)
	}
	items := rpc(t, s, "lead", "inbox.list", map[string]any{}).([]Delivery)
	if len(items) != 1 || items[0].ReplyTo != d.ID || items[0].ThreadID != d.ThreadID {
		t.Fatal(items)
	}
	d = send(t, s, "lead", "builder", "large")
	claim(t, s, "builder")
	if err := s.Finish(d.ID, "native", strings.Repeat("x", 129<<10), nil); err != nil {
		t.Fatal(err)
	}
	items = rpc(t, s, "builder", "inbox.list", map[string]any{}).([]Delivery)
	if items[1].Status != "failed" || items[1].Output != "" {
		t.Fatal("oversized runtime output retained")
	}
}

func TestWorkflowDowngradeAndCriteriaBounds(t *testing.T) {
	s, _, _ := setup(t)
	args := map[string]any{"to": "builder", "title": "test", "text": "inspect", "criteria": "evidence", "reviewer": "reviewer", "idempotencyKey": "task"}
	rpc(t, s, "lead", "tasks.assign", args)
	for _, id := range []string{"lead", "builder", "reviewer"} {
		denied(t, s, "operator", "sessions.policy", map[string]any{"id": id, "policy": "coordination"})
	}
	args["criteria"] = strings.Repeat("x", 17<<10)
	args["idempotencyKey"] = "large"
	denied(t, s, "lead", "tasks.assign", args)
}
