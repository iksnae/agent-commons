// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"
)

const testThread = "00000000-0000-0000-0000-000000000001"

var wakeEvent = []byte(`{"type":"inbox.available","messageId":"one","recipient":"lead"}`)

func TestWakeDurableDedupAndLock(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	w, err := openWake(context.Background(), dir, testThread, "token", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if other, err := openWake(context.Background(), dir, testThread, "token", io.Discard); err == nil {
		other.lock.Close()
		t.Fatal("duplicate watcher allowed")
	}
	calls := 0
	w.queue = func(context.Context, string, string) error { calls++; return nil }
	if _, err = w.Write(wakeEvent); err != nil {
		t.Fatal(err)
	}
	if _, err = w.Write(wakeEvent); err != nil || calls != 1 {
		t.Fatalf("%v calls=%d", err, calls)
	}
	w.lock.Close()
	w, err = openWake(context.Background(), dir, testThread, "token", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer w.lock.Close()
	w.queue = func(context.Context, string, string) error { t.Fatal("replayed after restart"); return nil }
	if _, err = w.Write(wakeEvent); err != nil {
		t.Fatal(err)
	}
}
func TestWakeUncertainDoesNotRetry(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	w, err := openWake(context.Background(), dir, testThread, "token", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	w.queue = func(context.Context, string, string) error { return errors.New("lost connection") }
	if _, err = w.Write(wakeEvent); err == nil {
		t.Fatal("failure hidden")
	}
	w.lock.Close()
	w, err = openWake(context.Background(), dir, testThread, "token", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer w.lock.Close()
	w.queue = func(context.Context, string, string) error { t.Fatal("uncertain attempt replayed"); return nil }
	if _, err = w.Write(wakeEvent); err == nil {
		t.Fatal("uncertainty hidden")
	}
}

func TestWakeCoalescesBatch(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	w, err := openWake(context.Background(), dir, testThread, "stable-identity", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer w.lock.Close()
	calls := 0
	w.queue = func(context.Context, string, string) error { calls++; return nil }
	if err := w.Notify([]availability{{"inbox.available", "a", "lead"}, {"inbox.available", "b", "lead"}}); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || w.records["a"].Status != "queued" || w.records["b"].Status != "queued" {
		t.Fatal("batch not coalesced")
	}
}
