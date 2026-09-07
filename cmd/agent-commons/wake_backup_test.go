// SPDX-License-Identifier: MPL-2.0

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWakePruneBackupFailureDoesNotChangeRecords(t *testing.T) {
	w := &wakeWriter{path: filepath.Join(t.TempDir(), "missing", "ledger.json"), records: map[string]wakeRecord{"kept": {Status: "queued"}}}
	var report wakeMaintenanceReport
	if err := w.prune(map[string]wakeRecord{}, &report); err == nil {
		t.Fatal("backup failure hidden")
	}
	if len(w.records) != 1 || report.Applied || report.Uncertain {
		t.Fatal("changed state despite backup failure")
	}
}

func TestWakePruneWriteFailurePreservesBackupAndReportsUncertainty(t *testing.T) {
	// Renaming a file over a directory reliably fails without permission tricks.
	path := filepath.Join(t.TempDir(), "ledger.json")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	w := &wakeWriter{path: path, records: map[string]wakeRecord{"retained": {Status: "queued"}}}
	var report wakeMaintenanceReport
	if err := w.prune(map[string]wakeRecord{}, &report); err == nil {
		t.Fatal("write failure hidden")
	}
	if report.Applied || !report.Uncertain || report.Backup == "" {
		t.Fatal("failure not distinguished from confirmed application", report)
	}
	if w.records["retained"].Status != "queued" {
		t.Fatal("failed replacement discarded in-memory suppression history")
	}
	var restored map[string]wakeRecord
	if err := privateReadLimit(report.Backup, &restored, maxWakeLedger); err != nil || restored["retained"].Status != "queued" {
		t.Fatal("recoverable backup missing", err)
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		t.Fatal("failed replacement altered target", err)
	}
}
