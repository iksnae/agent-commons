// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"context"
	"testing"
	"time"

	"agentcommons/internal/core"
)

type observedRun struct {
	identity string
	ctx      context.Context
}

type observingRunner struct{ started chan observedRun }

func (r observingRunner) Run(ctx context.Context, session core.Session, _ core.Delivery) (string, string, error) {
	r.started <- observedRun{session.ID, ctx}
	<-ctx.Done()
	return "", "", ctx.Err()
}

func TestScopeCancellationLeavesUnrelatedRunActive(t *testing.T) {
	s, target, _ := supervisorFixture(t)
	supervisorCall(t, s, "operator", "sessions.register", core.Session{ID: "other", Target: target, Mode: "managed", Runtime: "codex", Policy: "workflow"})
	other := supervisorCall(t, s, "lead", "messages.send", map[string]any{"to": "other", "text": "Project work", "idempotencyKey": "other"}).(core.Delivery)
	runner := observingRunner{started: make(chan observedRun, 2)}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, s, runner, time.Millisecond) }()
	defer func() { cancel(); awaitSupervisor(t, done) }()
	active := map[string]context.Context{}
	for len(active) < 2 {
		select {
		case run := <-runner.started:
			active[run.identity] = run.ctx
		case <-time.After(3 * time.Second):
			t.Fatal("concurrent runs did not start")
		}
	}
	supervisorCall(t, s, "reviewer", "teams.leave", map[string]any{"id": "product"})
	select {
	case <-active["builder"].Done():
	case <-time.After(3 * time.Second):
		t.Fatal("scoped run was not canceled")
	}
	if active["other"].Err() != nil || !s.RuntimeActive(other.ID) {
		t.Fatal("unrelated project work was canceled")
	}
}
