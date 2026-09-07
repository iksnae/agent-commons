// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"agentcommons/internal/codexlaunch"
)

type failedResumeBootstrap struct{ fakeBootstrap }

func (p *failedResumeBootstrap) Call(context.Context, string, any) (json.RawMessage, error) {
	return nil, errors.New("native unavailable")
}

func TestCodexResumeCheckRetainsBindingAndClosesFailedProcess(t *testing.T) {
	args, _ := codexPrepareFixture(t)
	start := func(_ context.Context, scope codexlaunch.Scope) (codexBootstrap, error) {
		return &fakeBootstrap{scope: scope}, nil
	}
	if err := runCodexPrepareWith(context.Background(), args, io.Discard, io.Discard, start); err != nil {
		t.Fatal(err)
	}
	process := &failedResumeBootstrap{}
	starts := 0
	var output bytes.Buffer
	err := runCodexResumeCheckWith(context.Background(), args, &output, io.Discard,
		func(context.Context, codexlaunch.Scope) (codexBootstrap, error) { starts++; return process, nil })
	if err == nil || !process.closed || starts != 1 {
		t.Fatal("failed process leaked or restarted", err)
	}
	var report codexResumeReport
	if err = json.Unmarshal(output.Bytes(), &report); err != nil || report.Verified || report.ThreadID != testThread {
		t.Fatal("failure report lost saved identity", err)
	}
	role, err := parseCodexRoleScope(context.Background(), "test", args, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if id, err := codexlaunch.LoadReady(report.Binding, role.Native); err != nil || id != testThread {
		t.Fatal("native failure damaged binding", err)
	}
}

func TestCodexResumeCheckRequiresSavedBindingBeforeStartup(t *testing.T) {
	args, connection := codexPrepareFixture(t)
	starts := 0
	var process *fakeBootstrap
	start := func(_ context.Context, scope codexlaunch.Scope) (codexBootstrap, error) {
		starts++
		process = &fakeBootstrap{scope: scope}
		return process, nil
	}
	if err := runCodexResumeCheckWith(context.Background(), args, io.Discard, io.Discard, start); err == nil || starts != 0 {
		t.Fatal("missing binding started a native process", err)
	}
	var prepared bytes.Buffer
	if err := runCodexPrepareWith(context.Background(), args, &prepared, io.Discard, start); err != nil {
		t.Fatal(err)
	}
	before, err := rpcCall[json.RawMessage](context.Background(), connection.client, "inbox.page", struct{}{})
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err = runCodexResumeCheckWith(context.Background(), args, &output, io.Discard, start); err != nil {
		t.Fatal(err)
	}
	var report codexResumeReport
	if err = json.Unmarshal(output.Bytes(), &report); err != nil || !report.Verified || report.ThreadID != testThread || starts != 2 || !process.closed {
		t.Fatal("resume check failed or left process open", err)
	}
	if bytes.Contains(output.Bytes(), []byte(connection.client.token)) {
		t.Fatal("resume report leaked credential")
	}
	otherHome := t.TempDir()
	if err = os.Chmod(otherHome, 0700); err != nil {
		t.Fatal(err)
	}
	wrongHome := append([]string(nil), args...)
	wrongHome[3] = otherHome
	if err = runCodexResumeCheckWith(context.Background(), wrongHome, io.Discard, io.Discard, start); err == nil || starts != 2 {
		t.Fatal("different Codex home started native process", err)
	}
	after, err := rpcCall[json.RawMessage](context.Background(), connection.client, "inbox.page", struct{}{})
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("resume check changed inbox", err)
	}
	if err = os.Remove(filepath.Join(report.Binding, "ready.json")); err != nil {
		t.Fatal(err)
	}
	if err = runCodexResumeCheckWith(context.Background(), args, io.Discard, io.Discard, start); err == nil || starts != 2 {
		t.Fatal("incomplete binding started native process", err)
	}
}
