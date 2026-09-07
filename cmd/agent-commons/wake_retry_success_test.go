// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"io"
	"os"
	"testing"
)

func TestWakeApprovedRetrySuccessRemainsSuppressedAfterRestart(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	w, err := openWake(context.Background(), dir, testThread, "role", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer w.lock.Close()
	original := wakeRecord{Status: "attempting", At: "original"}
	resolved, err := resolveWakeRecord(original, wakeRecordHash(original), "retry", "Reviewed interrupted dispatch")
	if err != nil {
		t.Fatal(err)
	}
	w.records["one"] = resolved
	calls := 0
	w.queue = func(context.Context, string, string) error { calls++; return nil }
	if _, err = w.Write(wakeEvent); err != nil || calls != 1 {
		t.Fatal("approved retry did not dispatch once", calls, err)
	}
	if err = w.lock.Close(); err != nil {
		t.Fatal(err)
	}
	w, err = openWake(context.Background(), dir, testThread, "role", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer w.lock.Close()
	w.queue = func(context.Context, string, string) error { calls++; return nil }
	if _, err = w.Write(wakeEvent); err != nil || calls != 1 {
		t.Fatal("successful retry replayed after restart", calls, err)
	}
	record := w.records["one"]
	if record.Status != "queued" || record.Resolution == nil || *record.Resolution != *resolved.Resolution {
		t.Fatal("saved success lost resolution evidence")
	}
}
