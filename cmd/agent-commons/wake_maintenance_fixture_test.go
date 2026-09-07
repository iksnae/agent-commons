// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"testing"
)

func wakeMaintenanceFixture(t *testing.T) ([]string, projectConnection, string) {
	t.Helper()
	state := onboardingService(t)
	data := onboardingCommand(t, "enroll", "--state", state, "--target", t.TempDir(), "--name", "lead", "--role", "lead")
	var enrolled struct {
		Config string `json:"config"`
	}
	if err := json.Unmarshal(data, &enrolled); err != nil {
		t.Fatal(err)
	}
	connection, err := openProjectConnection(enrolled.Config)
	if err != nil {
		t.Fatal(err)
	}
	page, err := rpcCall[wakeReceiptPage](context.Background(), connection.client, "inbox.page", struct{}{})
	if err != nil || len(page.Messages) != 2 {
		t.Fatal("missing welcome messages", err)
	}
	if _, err = rpcCall[json.RawMessage](context.Background(), connection.client, "inbox.acknowledge", map[string]string{"messageId": page.Messages[0].ID}); err != nil {
		t.Fatal(err)
	}
	socket, err := filepath.EvalSymlinks(connection.config.Socket)
	if err != nil {
		t.Fatal(err)
	}
	w, err := openWake(context.Background(), state, testThread, socket+"\x00"+connection.config.Identity, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer w.lock.Close()
	w.records = map[string]wakeRecord{page.Messages[0].ID: {Status: "queued"}, page.Messages[1].ID: {Status: "queued"}, "uncertain": {Status: "uncertain"}}
	if err = w.save(); err != nil {
		t.Fatal(err)
	}
	return []string{"wake-maintain", "--config", enrolled.Config, "--codex-thread", testThread}, connection, w.path
}
