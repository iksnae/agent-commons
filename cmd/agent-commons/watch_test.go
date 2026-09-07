// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"agentcommons/internal/core"
)

func TestWatchDeduplicatesWithoutAcknowledging(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	var out bytes.Buffer
	m := core.Delivery{ID: "one", To: "receiver", Text: "sensitive peer text"}
	err := watchInbox(ctx, 100*time.Millisecond, false, func(context.Context) ([]core.Delivery, error) {
		calls++
		if calls == 3 {
			cancel()
		}
		return []core.Delivery{m, {ID: "old", Acknowledged: true}}, nil
	}, &out)
	if !errors.Is(err, context.Canceled) || strings.Count(out.String(), "inbox.available") != 1 || strings.Contains(out.String(), "sensitive") || m.Acknowledged {
		t.Fatalf("err=%v output=%s", err, &out)
	}
}

func TestWatchOnceWaitsAndReplaysOnRestart(t *testing.T) {
	for i := 0; i < 2; i++ {
		calls := 0
		var out bytes.Buffer
		err := watchInbox(context.Background(), 100*time.Millisecond, true, func(context.Context) ([]core.Delivery, error) {
			calls++
			if calls == 1 {
				return nil, nil
			}
			return []core.Delivery{{ID: "pending", To: "me"}}, nil
		}, &out)
		if err != nil || calls != 2 || !strings.Contains(out.String(), "pending") {
			t.Fatalf("%v %s", err, &out)
		}
	}
}

func TestWatchCancellationAndReadFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := watchInbox(ctx, time.Second, false, func(context.Context) ([]core.Delivery, error) { t.Fatal("read after cancel"); return nil, nil }, io.Discard); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	want := errors.New("unavailable")
	if err := watchInbox(context.Background(), time.Second, false, func(context.Context) ([]core.Delivery, error) { return nil, want }, io.Discard); !errors.Is(err, want) {
		t.Fatal(err)
	}
	if err := watchInbox(context.Background(), 0, true, nil, io.Discard); err == nil {
		t.Fatal("zero interval allowed")
	}
}

func TestWatchCLIAndDiscovery(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"--help"}, {"methods"}} {
		var out bytes.Buffer
		if err := run(context.Background(), args, strings.NewReader(""), &out, io.Discard); err != nil || out.Len() == 0 {
			t.Fatalf("%v %v", args, err)
		}
	}
	if err := run(context.Background(), []string{"watch"}, strings.NewReader(""), io.Discard, io.Discard); err == nil {
		t.Fatal("operator credential fallback allowed")
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestWatchOutputFailureAndDeadline(t *testing.T) {
	if err := watchInbox(context.Background(), time.Second, true, func(context.Context) ([]core.Delivery, error) {
		return []core.Delivery{{ID: "one"}}, nil
	}, brokenWriter{}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := watchInbox(ctx, time.Second, true, func(context.Context) ([]core.Delivery, error) { return nil, nil }, io.Discard); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}
