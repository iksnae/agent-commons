// SPDX-License-Identifier: MPL-2.0

package core

import (
	"testing"
	"time"
)

func TestSupervisorHealthDoesNotSurviveRestart(t *testing.T) {
	s, dir, _ := setup(t)
	if s.supervisorHealth(time.Now()).State != "not_observed" {
		t.Fatal("unstarted supervisor reported healthy")
	}
	if err := s.StartSupervisor(250 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if err := s.StartSupervisor(time.Second); err == nil {
		t.Fatal("duplicate supervisor allowed")
	}
	if s.supervisorHealth(time.Now()).State != "responsive" {
		t.Fatal("fresh supervisor not observed")
	}
	if s.supervisorHealth(s.supervisor.seen.Add(2*time.Second)).State != "stale" {
		t.Fatal("old heartbeat still responsive")
	}
	s.SupervisorHeartbeat()
	s.StopSupervisor()
	if s.supervisorHealth(time.Now()).State != "stopped" {
		t.Fatal("stopped supervisor still responsive")
	}
	s.Close()
	reopened, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if reopened.supervisorHealth(time.Now()).State != "not_observed" {
		t.Fatal("restart inherited old heartbeat")
	}
}
