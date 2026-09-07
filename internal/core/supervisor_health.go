// SPDX-License-Identifier: MPL-2.0

package core

import (
	"errors"
	"time"
)

type supervisorObservation struct {
	running  bool
	seen     time.Time
	interval time.Duration
}

type SupervisorHealth struct {
	State      string `json:"state"`
	LastSeen   string `json:"lastSeen,omitempty"`
	AgeMillis  int64  `json:"ageMillis"`
	PollMillis int64  `json:"pollMillis"`
}

// StartSupervisor reserves the in-process supervisor slot. Observations are
// deliberately not persisted: a service restart cannot inherit a live heartbeat.
func (s *Service) StartSupervisor(interval time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.supervisor.running || interval <= 0 {
		return errors.New("supervisor unavailable, already running, or invalid interval")
	}
	s.supervisor = supervisorObservation{running: true, seen: time.Now(), interval: interval}
	return nil
}

func (s *Service) SupervisorHeartbeat() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed && s.supervisor.running {
		s.supervisor.seen = time.Now()
	}
}

func (s *Service) StopSupervisor() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.supervisor.running = false
}

func (s *Service) supervisorHealth(now time.Time) SupervisorHealth {
	observed := s.supervisor
	result := SupervisorHealth{State: "not_observed"}
	if observed.seen.IsZero() {
		return result
	}
	result.LastSeen = observed.seen.UTC().Format(time.RFC3339Nano)
	result.AgeMillis = max(0, now.Sub(observed.seen).Milliseconds())
	result.PollMillis = observed.interval.Milliseconds()
	result.State = "stopped"
	if observed.running {
		result.State = "responsive"
		if now.Sub(observed.seen) > 3*observed.interval+time.Second {
			result.State = "stale"
		}
	}
	return result
}
