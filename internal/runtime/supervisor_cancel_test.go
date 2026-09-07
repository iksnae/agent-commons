// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"context"
	"sync"
	"testing"
	"time"

	"agentcommons/internal/core"
)

type canceledSuccessRunner struct {
	started, canceled chan struct{}
	release           chan struct{}
}

func TestParentCancellationWithholdsIncorrectRunnerSuccess(t *testing.T) {
	s, _, task := supervisorFixture(t)
	runner := canceledSuccessRunner{make(chan struct{}), make(chan struct{}), make(chan struct{}, 1)}
	runner.release <- struct{}{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, s, runner, time.Millisecond) }()
	var stopped sync.Once
	stop := func() { stopped.Do(func() { cancel(); awaitSupervisor(t, done) }) }
	defer stop()
	select {
	case <-runner.started:
	case <-time.After(3 * time.Second):
		t.Fatal("runner did not start")
	}
	stop()
	got := supervisorCall(t, s, "lead", "tasks.get", map[string]any{"id": task.ID}).(core.Task)
	if got.Status != "failed" || got.Output != "" || got.Revision != 0 {
		t.Fatal("parent cancellation allowed successful submission")
	}
}

func (r canceledSuccessRunner) Run(ctx context.Context, _ core.Session, _ core.Delivery) (string, string, error) {
	close(r.started)
	<-ctx.Done()
	close(r.canceled)
	<-r.release
	return "retained-native-root", "must not submit", nil
}

func TestSupervisorCancellationStaysFailedAfterTeamRejoin(t *testing.T) {
	s, _, task := supervisorFixture(t)
	runner := canceledSuccessRunner{make(chan struct{}), make(chan struct{}), make(chan struct{}, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, s, runner, time.Millisecond) }()
	defer func() {
		cancel()
		select {
		case runner.release <- struct{}{}:
		default:
		}
		awaitSupervisor(t, done)
	}()
	select {
	case <-runner.started:
	case <-time.After(3 * time.Second):
		t.Fatal("runner did not start")
	}
	supervisorCall(t, s, "reviewer", "teams.leave", map[string]any{"id": "product"})
	select {
	case <-runner.canceled:
	case <-time.After(3 * time.Second):
		t.Fatal("membership withdrawal did not cancel runner")
	}
	supervisorCall(t, s, "reviewer", "teams.join", map[string]any{"id": "product"})
	// The parent remains active: only the per-delivery cancellation can prevent
	// this incorrectly successful runner result from being submitted.
	runner.release <- struct{}{}
	deadline := time.After(3 * time.Second)
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		got := supervisorCall(t, s, "lead", "tasks.get", map[string]any{"id": task.ID}).(core.Task)
		if got.Status == "failed" && got.Output == "" && got.Revision == 0 {
			return
		}
		if got.Output != "" || got.Revision != 0 {
			t.Fatal("canceled runner submitted after rejoin")
		}
		select {
		case <-deadline:
			t.Fatal("canceled run did not finish")
		case <-ticker.C:
		}
	}
}
