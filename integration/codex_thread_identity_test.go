// SPDX-License-Identifier: MPL-2.0

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type nativeThreadIdentity struct {
	ID             string  `json:"id"`
	SessionID      string  `json:"sessionId"`
	CWD            string  `json:"cwd"`
	ForkedFromID   *string `json:"forkedFromId"`
	ParentThreadID *string `json:"parentThreadId"`
}

func TestNativeCodexDistinguishesResumedAndForkedThreadIdentity(t *testing.T) {
	if os.Getenv("AGENT_COMMONS_CODEX_HOOK") != "1" {
		t.Skip("native Codex identity test is opt-in")
	}
	configDir, target := t.TempDir(), t.TempDir()
	request := codexFixture(t, configDir, target)
	root := decodeNativeIdentity(t, request("thread/start", map[string]any{
		"cwd": target, "approvalPolicy": "never", "sandbox": "read-only",
	}))
	canonicalTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	actualTarget, err := filepath.EvalSymlinks(root.CWD)
	if err != nil || actualTarget != canonicalTarget || root.ID == "" || root.SessionID == "" || root.ForkedFromID != nil || root.ParentThreadID != nil {
		t.Fatal("new root identity is incomplete or has unexpected ancestry", err)
	}
	// Materialize history through the native API without starting a model turn.
	request("thread/inject_items", map[string]any{"threadId": root.ID, "items": []any{
		map[string]any{"type": "message", "role": "user", "content": []any{
			map[string]string{"type": "input_text", "text": "Disposable identity fixture; no task requested."},
		}},
	}})
	resumed := decodeNativeIdentity(t, request("thread/resume", map[string]any{"threadId": root.ID}))
	if resumed.ID != root.ID || resumed.SessionID != root.SessionID || resumed.ForkedFromID != nil || resumed.ParentThreadID != nil {
		t.Fatal("resume changed root identity or ancestry")
	}
	fork := decodeNativeIdentity(t, request("thread/fork", map[string]any{"threadId": root.ID}))
	if fork.ID == "" || fork.ID == root.ID || fork.ForkedFromID == nil || *fork.ForkedFromID != root.ID || fork.ParentThreadID != nil {
		t.Fatal("fork lacks a distinct thread ID and explicit fork ancestry")
	}
	stored := decodeNativeIdentity(t, request("thread/read", map[string]any{"threadId": fork.ID, "includeTurns": false}))
	if stored.ID != fork.ID || stored.ForkedFromID == nil || *stored.ForkedFromID != root.ID {
		t.Fatal("thread/read lost fork ancestry")
	}
	t.Log("native start/resume/fork/read distinguish exact thread IDs and fork ancestry; no model turn or role attachment")
}

func decodeNativeIdentity(t *testing.T, data json.RawMessage) nativeThreadIdentity {
	t.Helper()
	var response struct {
		Thread nativeThreadIdentity `json:"thread"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatal(err)
	}
	var fields struct {
		Thread map[string]json.RawMessage `json:"thread"`
	}
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"id", "sessionId", "cwd", "forkedFromId", "parentThreadId"} {
		if _, ok := fields.Thread[key]; !ok {
			t.Fatalf("native identity metadata omits %s", key)
		}
	}
	return response.Thread
}
