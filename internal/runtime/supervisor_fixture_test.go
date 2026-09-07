// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"encoding/json"
	"testing"
	"time"

	"agentcommons/internal/core"
)

func supervisorCall(t *testing.T, s *core.Service, actor, method string, params any) any {
	t.Helper()
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.Call(actor, method, raw)
	if err != nil {
		t.Fatal(method, err)
	}
	return result
}

func supervisorFixture(t *testing.T) (*core.Service, string, core.Task) {
	t.Helper()
	s, err := core.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	target := t.TempDir()
	supervisorCall(t, s, "operator", "teams.create", map[string]any{"id": "product", "target": target, "title": "Team", "text": "Review work"})
	for _, id := range []string{"lead", "builder", "reviewer"} {
		mode := "manual"
		if id == "builder" {
			mode = "managed"
		}
		supervisorCall(t, s, "operator", "sessions.register", core.Session{ID: id, Target: target, Mode: mode, Runtime: "codex", Policy: "workflow"})
		supervisorCall(t, s, "operator", "teams.invite", map[string]any{"id": "product", "target": target, "to": id})
		supervisorCall(t, s, id, "teams.join", map[string]any{"id": "product"})
	}
	d, err := s.Claim("builder")
	if err != nil || d == nil {
		t.Fatal("invitation missing", err)
	}
	if err := s.Finish(d.ID, "", "Joined", nil); err != nil {
		t.Fatal(err)
	}
	task := supervisorCall(t, s, "lead", "tasks.assign", map[string]any{"teamId": "product", "to": "builder", "title": "Inspect", "text": "Work", "criteria": "Evidence", "reviewer": "reviewer", "idempotencyKey": "work"}).(core.Task)
	return s, target, task
}

func awaitSupervisor(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("supervisor did not stop")
	}
}
