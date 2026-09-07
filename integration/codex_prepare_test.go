// SPDX-License-Identifier: MPL-2.0

package integration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"agentcommons/internal/codexlaunch"
)

type nativeCaller func(string, any) json.RawMessage

func (call nativeCaller) Call(_ context.Context, method string, params any) (json.RawMessage, error) {
	return call(method, params), nil
}

func TestNativeCodexPreparesRecoverableRoot(t *testing.T) {
	if os.Getenv("AGENT_COMMONS_CODEX_HOOK") != "1" {
		t.Skip("native Codex preparation test is opt-in")
	}
	home, target := t.TempDir(), t.TempDir()
	target, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	if err = os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parent, "binding")
	request, stop := codexFixtureWithStop(t, home, target)
	scope := codexlaunch.Scope{Identity: "disposable-lead", Target: target, Home: home}
	journal := codexlaunch.NewDirectoryJournal(path)
	t.Cleanup(func() { stop(); _ = journal.Close() })
	id, err := codexlaunch.Prepare(context.Background(), nativeCaller(request), journal, scope)
	if err != nil {
		t.Fatal(err)
	}
	stop()
	if err = journal.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = codexlaunch.Prepare(context.Background(), nativeCaller(func(string, any) json.RawMessage {
		t.Fatal("existing preparation attempted another native call")
		return nil
	}), codexlaunch.NewDirectoryJournal(path), scope); err == nil {
		t.Fatal("existing binding was silently replaced")
	}
	restarted := codexFixture(t, home, target)
	saved, err := codexlaunch.LoadReady(path, scope)
	if err != nil || saved != id {
		t.Fatal("cannot reload matching ready binding", err)
	}
	if err = codexlaunch.Resume(context.Background(), nativeCaller(restarted), scope, saved); err != nil {
		t.Fatal("fresh server did not resume verified prepared root", err)
	}
	var ready struct {
		ThreadID string `json:"threadId"`
	}
	data, err := os.ReadFile(filepath.Join(path, "ready.json"))
	if err != nil || json.Unmarshal(data, &ready) != nil || ready.ThreadID != id {
		t.Fatal("saved binding differs from native root", err)
	}
	t.Log("prepared root resumed through a fresh native app-server; durable binding retained; no model turn, hook trust or role attachment")
}
