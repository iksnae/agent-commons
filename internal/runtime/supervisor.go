// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"context"
	"errors"
	"sync"
	"time"

	"agentcommons/internal/core"
)

type activeRun struct {
	deliveryID string
	cancel     context.CancelCauseFunc
}

type supervisor struct {
	service *core.Service
	runner  Runner
	mu      sync.Mutex
	active  map[string]activeRun
	wg      sync.WaitGroup
	errors  chan error
}

// Serve polls durable state; only explicitly claimed work starts a runner.
func Serve(ctx context.Context, svc *core.Service, runner Runner, interval time.Duration) error {
	ctx, cancel := context.WithCancel(ctx)
	s := supervisor{service: svc, runner: runner, active: map[string]activeRun{}, errors: make(chan error, 1)}
	if interval <= 0 {
		interval = 250 * time.Millisecond
	}
	if err := svc.StartSupervisor(interval); err != nil {
		cancel()
		return err
	}
	defer func() { cancel(); s.wg.Wait(); svc.StopSupervisor() }()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		svc.SupervisorHeartbeat()
		s.cancelInactive()
		for _, session := range svc.Sessions() {
			if err := ctx.Err(); err != nil {
				return nil
			}
			if err := s.start(ctx, session); err != nil {
				return err
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case err := <-s.errors:
			return err
		case <-ticker.C:
		}
	}
}

func (s *supervisor) cancelInactive() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, run := range s.active {
		if !s.service.RuntimeActive(run.deliveryID) {
			run.cancel(errors.New("managed delivery no longer active or team access withdrawn"))
		}
	}
}

func (s *supervisor) start(ctx context.Context, session core.Session) error {
	s.mu.Lock()
	_, busy := s.active[session.ID]
	s.mu.Unlock()
	if session.Mode != "managed" || busy {
		return nil
	}
	d, err := s.service.Claim(session.ID)
	if err != nil || d == nil {
		return err
	}
	token, err := s.service.Token(session.ID)
	if err != nil {
		return errors.Join(err, s.service.Finish(d.ID, "", "", err))
	}
	runCtx, cancel := context.WithCancelCause(context.WithValue(ctx, credentialKey{}, token))
	s.mu.Lock()
	s.active[session.ID] = activeRun{deliveryID: d.ID, cancel: cancel}
	s.mu.Unlock()
	s.wg.Add(1)
	go s.run(runCtx, cancel, session, *d)
	return nil
}

func (s *supervisor) run(ctx context.Context, cancel context.CancelCauseFunc, session core.Session, d core.Delivery) {
	defer s.wg.Done()
	defer cancel(nil)
	id, output, runErr := s.runner.Run(ctx, session, d)
	// Serialize completion with scope cancellation so a cancellation already
	// observed by this supervisor cannot race a successful result commit.
	s.mu.Lock()
	defer s.mu.Unlock()
	if cause := context.Cause(ctx); cause != nil {
		runErr, output = errors.Join(core.ErrRuntimeCanceled, cause, runErr), ""
	}
	if err := s.service.Finish(d.ID, id, output, runErr); err != nil {
		select {
		case s.errors <- err:
		default:
		}
	}
	delete(s.active, session.ID)
}
