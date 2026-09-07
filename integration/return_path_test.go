// SPDX-License-Identifier: MPL-2.0

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"agentcommons/internal/core"
	runtimes "agentcommons/internal/runtime"
)

type receiptRunner struct {
	mu   sync.Mutex
	runs []core.Delivery
}

func (r *receiptRunner) Run(_ context.Context, s core.Session, d core.Delivery) (string, string, error) {
	r.mu.Lock()
	r.runs = append(r.runs, d)
	r.mu.Unlock()
	return "test-session-" + s.ID, "received " + d.Text, nil
}

func invoke(t *testing.T, s *core.Service, actor, method string, params any) any {
	t.Helper()
	p, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	v, err := s.Call(actor, method, p)
	if err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	return v
}

func register(t *testing.T, s *core.Service, id, target, runtime string) {
	t.Helper()
	invoke(t, s, "operator", "sessions.register", map[string]any{
		"id": id, "target": target, "team": "publisher", "role": "coordinator",
		"runtime": runtime, "mode": "managed",
	})
}

// A result must wake the original assigning session after the service restarts,
// without any caller polling the LLM or submitting a new user prompt.
func TestQueuedWorkSurvivesRestartAndWakesRequester(t *testing.T) {
	dir, target := t.TempDir(), t.TempDir()
	s, err := core.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	register(t, s, "lead", target, "codex")
	register(t, s, "worker", target, "claude")
	params := map[string]any{"to": "worker", "text": "KP-RETURN-RESTART", "idempotencyKey": "request-1"}
	invoke(t, s, "lead", "messages.send", params)
	invoke(t, s, "lead", "messages.send", params)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = core.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	runner := &receiptRunner{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- runtimes.Serve(ctx, s, runner, 5*time.Millisecond) }()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-deadline.C:
			t.Fatal("requester was not automatically woken")
		case <-tick.C:
			runner.mu.Lock()
			runs := append([]core.Delivery(nil), runner.runs...)
			runner.mu.Unlock()
			if len(runs) < 2 {
				continue
			}
			if len(runs) != 2 || runs[0].To != "worker" || runs[1].To != "lead" || runs[1].Kind != "result" {
				t.Fatalf("wrong route or duplicate execution: %+v", runs)
			}
			cancel()
			if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			// Finishing a result must not manufacture another result.
			if d, err := s.Claim("worker"); err != nil || d != nil {
				t.Fatalf("reply loop remains: delivery=%+v error=%v", d, err)
			}
			return
		}
	}
}
