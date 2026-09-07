// SPDX-License-Identifier: MPL-2.0

package core

import (
	"testing"
	"time"
)

func TestStableIdentityAttachments(t *testing.T) {
	s, dir, target := setup(t)
	identity := Session{ID: "stable", Name: "lead", Role: "lead", Target: target, Runtime: "manual", Mode: "manual"}
	rpc(t, s, "operator", "sessions.enroll", identity)
	first := rpc(t, s, "stable", "sessions.attach", map[string]any{"nativeId": "claude-1", "runtime": "claude", "target": target}).(Attachment)
	denied(t, s, "stable", "sessions.attach", map[string]any{"nativeId": "codex-1", "runtime": "codex", "target": target})
	denied(t, s, "stable", "sessions.renew", map[string]any{"nativeId": "claude-1", "leaseId": "wrong"})
	s.Close()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rpc(t, s, "stable", "sessions.renew", map[string]any{"nativeId": "claude-1", "leaseId": first.LeaseID})
	rpc(t, s, "stable", "sessions.detach", map[string]any{"nativeId": "claude-1", "leaseId": first.LeaseID})
	next := rpc(t, s, "stable", "sessions.attach", map[string]any{"nativeId": "codex-1", "runtime": "codex", "target": target}).(Attachment)
	if next.LeaseID == first.LeaseID || next.Runtime != "codex" {
		t.Fatal(next)
	}
	denied(t, s, "stable", "sessions.detach", map[string]any{"nativeId": "claude-1", "leaseId": first.LeaseID})
	v := s.data.Sessions["stable"]
	v.Attachment.ExpiresAt = time.Now().Unix() - 1
	s.data.Sessions[v.ID] = v
	denied(t, s, "stable", "sessions.renew", map[string]any{"nativeId": "codex-1", "leaseId": next.LeaseID})
	rpc(t, s, "stable", "sessions.attach", map[string]any{"nativeId": "claude-2", "runtime": "claude", "target": target})
}

func TestEnrollmentIdempotencyAndIdentityKey(t *testing.T) {
	s, _, target := setup(t)
	p := Session{ID: "stable", Name: "lead", Role: "lead", Target: target, Runtime: "manual", Mode: "manual"}
	a := rpc(t, s, "operator", "sessions.enroll", p).(map[string]any)
	p.Runtime = "codex"
	b := rpc(t, s, "operator", "sessions.enroll", p).(map[string]any)
	if a["token"] != b["token"] {
		t.Fatal("credential changed")
	}
	p.ID = "duplicate"
	denied(t, s, "operator", "sessions.enroll", p)
	denied(t, s, "lead", "sessions.enroll", p)
}
