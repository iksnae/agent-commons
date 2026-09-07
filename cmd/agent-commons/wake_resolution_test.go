// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"
)

func TestWakeResolutionRequiresExactUncertainRecord(t *testing.T) {
	record := wakeRecord{Status: "uncertain", At: "first"}
	hash := wakeRecordHash(record)
	for _, decision := range []string{"retry", "suppress"} {
		resolved, err := resolveWakeRecord(record, hash, decision, "Operator reviewed the notification history.")
		if err != nil || resolved.Resolution == nil || resolved.Resolution.PreviousHash != hash {
			t.Fatal("resolution evidence missing", err)
		}
		if _, err = resolveWakeRecord(resolved, hash, decision, "same request"); err == nil {
			t.Fatal("stale resolution replayed")
		}
	}
	if _, err := resolveWakeRecord(record, "stale", "retry", "reviewed"); err == nil {
		t.Fatal("stale hash accepted")
	}
	if _, err := resolveWakeRecord(record, hash, "retry", ""); err == nil {
		t.Fatal("missing evidence accepted")
	}
	queued := wakeRecord{Status: "queued"}
	if _, err := resolveWakeRecord(queued, wakeRecordHash(queued), "retry", "reviewed"); err == nil {
		t.Fatal("completed notification reopened")
	}
}

func TestWakeResolutionSurvivesRestartAndGatesRetry(t *testing.T) {
	for _, decision := range []string{"retry", "suppress"} {
		t.Run(decision, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Chmod(dir, 0700); err != nil {
				t.Fatal(err)
			}
			w, err := openWake(context.Background(), dir, testThread, "role", io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			original := wakeRecord{Status: "uncertain", At: "original"}
			resolved, err := resolveWakeRecord(original, wakeRecordHash(original), decision, "Operator decision")
			if err != nil {
				t.Fatal(err)
			}
			w.records["one"] = resolved
			if err = w.save(); err != nil {
				t.Fatal(err)
			}
			_ = w.lock.Close()
			w, err = openWake(context.Background(), dir, testThread, "role", io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			defer w.lock.Close()
			calls := 0
			w.queue = func(context.Context, string, string) error { calls++; return errors.New("uncertain again") }
			_, err = w.Write(wakeEvent)
			if decision == "suppress" && (err != nil || calls != 0 || w.records["one"].Status != "suppressed") {
				t.Fatal("suppression dispatched", err)
			}
			if decision == "retry" {
				if err == nil || calls != 1 || w.records["one"].Status != "uncertain" {
					t.Fatal("approved retry not attempted", err)
				}
				if _, err = w.Write(wakeEvent); err == nil || calls != 1 {
					t.Fatal("failed retry repeated automatically")
				}
			}
			if w.records["one"].Resolution == nil || w.records["one"].Resolution.Evidence != "Operator decision" {
				t.Fatal("dispatch discarded operator evidence")
			}
		})
	}
}
