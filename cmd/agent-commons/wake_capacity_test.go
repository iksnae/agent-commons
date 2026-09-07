// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

func TestFullWakeLedgerPreservesStateAndDoesNotDispatch(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	w, err := openWake(context.Background(), dir, testThread, "role", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.lock.Close() })
	w.records["retained"] = wakeRecord{Status: "queued"}
	base, err := json.Marshal(w.records)
	if err != nil {
		t.Fatal(err)
	}
	w.records["retained"] = wakeRecord{Status: "queued", At: strings.Repeat("x", maxWakeLedger-len(base))}
	if err = w.save(); err != nil {
		t.Fatal("exact capacity rejected", err)
	}
	before, err := os.ReadFile(w.path)
	if err != nil || len(before) != maxWakeLedger {
		t.Fatal("fixture did not reach exact capacity", err)
	}
	w.queue = func(context.Context, string, string) error { t.Fatal("dispatched without durable intent"); return nil }
	if err = w.Notify([]availability{{"inbox.available", "new", "lead"}}); err == nil {
		t.Fatal("overflow accepted")
	}
	after, err := os.ReadFile(w.path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("overflow changed durable ledger", err)
	}
	if err = w.lock.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := openWake(context.Background(), dir, testThread, "role", io.Discard)
	if err != nil {
		t.Fatal("capacity-limited ledger cannot reopen", err)
	}
	defer reopened.lock.Close()
	if len(reopened.records) != 1 || reopened.records["retained"].Status != "queued" {
		t.Fatal("restart lost wake history")
	}
}
