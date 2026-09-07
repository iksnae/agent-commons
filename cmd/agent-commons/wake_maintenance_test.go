// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestWakeMaintenancePreviewsBacksUpAndPreservesInbox(t *testing.T) {
	args, connection, path := wakeMaintenanceFixture(t)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	inbox, err := rpcCall[json.RawMessage](context.Background(), connection.client, "inbox.page", struct{}{})
	if err != nil {
		t.Fatal(err)
	}
	runMaintenance := func(extra ...string) wakeMaintenanceReport {
		t.Helper()
		var output bytes.Buffer
		if err := run(context.Background(), append(append([]string{}, args...), extra...), nil, &output, io.Discard); err != nil {
			t.Fatal(err)
		}
		var report wakeMaintenanceReport
		if err := json.Unmarshal(output.Bytes(), &report); err != nil {
			t.Fatal(err)
		}
		return report
	}
	preview := runMaintenance()
	if preview.Eligible != 1 || preview.Remaining != 2 || preview.Applied || preview.Backup != "" {
		t.Fatal("wrong preview", preview)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("preview changed ledger", err)
	}
	result := runMaintenance("--apply", "--acknowledge-replay-risk")
	if !result.Applied || result.Eligible != 1 || result.Backup == "" {
		t.Fatal("maintenance not applied", result)
	}
	backup, err := os.ReadFile(result.Backup)
	if err != nil || !bytes.Equal(backup, before) {
		t.Fatal("backup does not preserve suppression history", err)
	}
	info, err := os.Stat(result.Backup)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("backup not private", err)
	}
	var records map[string]wakeRecord
	if err = privateReadLimit(path, &records, maxWakeLedger); err != nil || len(records) != 2 || records["uncertain"].Status != "uncertain" {
		t.Fatal("unsafe prune", err)
	}
	current, err := rpcCall[json.RawMessage](context.Background(), connection.client, "inbox.page", struct{}{})
	if err != nil || !bytes.Equal(inbox, current) {
		t.Fatal("maintenance changed inbox", err)
	}
	repeated := runMaintenance("--apply", "--acknowledge-replay-risk")
	if repeated.Applied || repeated.Eligible != 0 || repeated.Backup != "" {
		t.Fatal("repeat maintenance was not a no-op")
	}
}

func TestWakeMaintenanceRefusesMissingRiskConfirmationAndActiveWatcher(t *testing.T) {
	args, connection, path := wakeMaintenanceFixture(t)
	if err := run(context.Background(), append(args, "--apply"), nil, io.Discard, io.Discard); err == nil {
		t.Fatal("missing risk confirmation accepted")
	}
	socket, err := filepath.EvalSymlinks(connection.config.Socket)
	if err != nil {
		t.Fatal(err)
	}
	w, err := openWake(context.Background(), connection.config.State, testThread, socket+"\x00"+connection.config.Identity, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer w.lock.Close()
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = run(context.Background(), args, nil, io.Discard, io.Discard); err == nil {
		t.Fatal("maintenance accepted active watcher")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("refusal changed ledger", err)
	}
}
