// SPDX-License-Identifier: MPL-2.0

package core

import "testing"

func TestRuntimeStatusReportsInterruptedWorkAfterRestart(t *testing.T) {
	s, dir, _ := setup(t)
	send(t, s, "lead", "builder", "interrupted")
	claim(t, s, "builder")
	s.Close()
	reopened, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	page := rpc(t, reopened, "builder", "runtime.status", nil).(RuntimeStatusPage)
	got := page.Sessions[0]
	if got.Interrupted != 1 || got.Running != 0 || got.Ready != 0 || got.Busy || page.Supervisor.State != "not_observed" {
		t.Fatal("restart status implies replay or stale live execution")
	}
}

func TestRuntimeStatusDoesNotLabelPolicyBlockedTaskReady(t *testing.T) {
	s, _, target := setup(t)
	joinWorkTeam(t, s, target, "product", "lead", "builder", "reviewer", "red")
	rpc(t, s, "lead", "tasks.assign", teamAssignment())
	// Model inconsistent imported state; the public policy API already refuses
	// this downgrade while the identity has task duties.
	session := s.data.Sessions["builder"]
	session.Policy = "coordination"
	s.data.Sessions[session.ID] = session
	page := rpc(t, s, "builder", "runtime.status", nil).(RuntimeStatusPage)
	if got := page.Sessions[0]; got.Ready != 0 || got.BlockedPolicy != 1 {
		t.Fatal("task forbidden by policy reported ready")
	}
}
