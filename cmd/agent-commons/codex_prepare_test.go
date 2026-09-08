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

	"agentcommons/internal/codexlaunch"
)

type fakeBootstrap struct {
	scope  codexlaunch.Scope
	closed bool
}

func (p *fakeBootstrap) Close() error { p.closed = true; return nil }
func (p *fakeBootstrap) Call(context.Context, string, any) (json.RawMessage, error) {
	data, err := json.Marshal(map[string]any{"thread": map[string]any{"id": testThread, "cwd": p.scope.Target, "forkedFromId": nil, "parentThreadId": nil}})
	return data, err
}

func TestCodexPrepareVerifiesRoleAndPreservesCoordinationState(t *testing.T) {
	args, connection := codexPrepareFixture(t)
	before, err := rpcCall[json.RawMessage](context.Background(), connection.client, "inbox.page", struct{}{})
	if err != nil {
		t.Fatal(err)
	}
	starts := 0
	var process *fakeBootstrap
	start := func(_ context.Context, scope codexlaunch.Scope) (codexBootstrap, error) {
		starts++
		process = &fakeBootstrap{scope: scope}
		return process, nil
	}
	var output bytes.Buffer
	if err = runCodexPrepareWith(context.Background(), args, &output, io.Discard, start); err != nil {
		t.Fatal(err)
	}
	if starts != 1 || !process.closed {
		t.Fatal("native process ownership lost")
	}
	if bytes.Contains(output.Bytes(), []byte(connection.client.token)) {
		t.Fatal("credential leaked")
	}
	var report codexPreparationReport
	if err = json.Unmarshal(output.Bytes(), &report); err != nil || !report.Prepared || report.ThreadID != testThread {
		t.Fatal("bad preparation report", err)
	}
	if err = runCodexPrepareWith(context.Background(), args, io.Discard, io.Discard, start); err == nil || starts != 1 {
		t.Fatal("repeated preparation started native process")
	}
	after, err := rpcCall[json.RawMessage](context.Background(), connection.client, "inbox.page", struct{}{})
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("preparation changed inbox", err)
	}
	connection.config.Role = "wrong-role"
	data, err := json.Marshal(connection.config)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(args[1], data, 0600); err != nil {
		t.Fatal(err)
	}
	if err = runCodexPrepareWith(context.Background(), args, io.Discard, io.Discard, start); err == nil || starts != 1 {
		t.Fatal("unverified role started Codex")
	}
}

func codexPrepareFixture(t *testing.T) ([]string, projectConnection) {
	t.Helper()
	state := onboardingService(t)
	var enrolled struct {
		Config string `json:"config"`
	}
	if err := json.Unmarshal(onboardingCommand(t, "enroll", "--json", "--state", state, "--target", t.TempDir(), "--name", "lead", "--role", "lead"), &enrolled); err != nil {
		t.Fatal(err)
	}
	connection, err := openProjectConnection(enrolled.Config)
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	if err = os.Chmod(home, 0700); err != nil {
		t.Fatal(err)
	}
	home, err = filepath.EvalSymlinks(home)
	if err != nil {
		t.Fatal(err)
	}
	return []string{"--config", enrolled.Config, "--codex-home", home}, connection
}
