// SPDX-License-Identifier: MPL-2.0

package core

import (
	"errors"
	"testing"
)

func TestFinishCannotReplaceBoundRuntimeOnSuccessOrFailure(t *testing.T) {
	for _, providerErr := range []error{nil, errors.New("provider failed")} {
		name := "reported-success"
		if providerErr != nil {
			name = "reported-failure"
		}
		t.Run(name, func(t *testing.T) {
			s, dir, _ := setup(t)
			send(t, s, "lead", "builder", "first")
			first := claim(t, s, "builder")
			if err := s.Finish(first.ID, "bound-root", "first result", nil); err != nil {
				t.Fatal(err)
			}
			send(t, s, "lead", "builder", "second")
			second := claim(t, s, "builder")
			if err := s.Finish(second.ID, "different-root", "wrong session output", providerErr); err != nil {
				t.Fatal("failed outcome should be durably recorded", err)
			}
			s.Close()
			reopened, err := New(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			for _, session := range reopened.Sessions() {
				if session.ID == "builder" && (session.RuntimeSessionID != "bound-root" || session.Busy) {
					t.Fatal("native binding replaced or left busy", session)
				}
			}
			inbox := rpc(t, reopened, "builder", "inbox.list", nil).([]Delivery)
			if inbox[1].Status != "failed" || inbox[1].Output != "" || inbox[1].Error == "" {
				t.Fatal("mismatched result was not quarantined")
			}
			if len(rpc(t, reopened, "lead", "inbox.list", nil).([]Delivery)) != 1 {
				t.Fatal("mismatched result returned as successful work")
			}
			if d, err := reopened.Claim("builder"); err != nil || d != nil {
				t.Fatal("mismatched work replayed without operator decision", err)
			}
		})
	}
}

func TestMismatchedRuntimeCannotSubmitTaskForReview(t *testing.T) {
	s, _, _ := setup(t)
	send(t, s, "lead", "builder", "bind")
	if err := s.Finish(claim(t, s, "builder").ID, "bound-root", "ready", nil); err != nil {
		t.Fatal(err)
	}
	task := rpc(t, s, "lead", "tasks.assign", map[string]any{
		"to": "builder", "title": "inspect", "text": "inspect", "criteria": "evidence",
		"reviewer": "reviewer", "redTeam": "red", "idempotencyKey": "task",
	}).(Task)
	if err := s.Finish(claim(t, s, "builder").ID, "different-root", "wrong result", nil); err != nil {
		t.Fatal(err)
	}
	got := rpc(t, s, "lead", "tasks.get", map[string]string{"id": task.ID}).(Task)
	if got.Status != "failed" || got.Output != "" || got.Revision != 0 || len(got.Reviews) != 0 {
		t.Fatal("wrong-session result entered review workflow", got)
	}
	denied(t, s, "lead", "tasks.accept", map[string]string{"id": task.ID})
}
