// SPDX-License-Identifier: MPL-2.0

package transport

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"agentcommons/internal/core"
)

func TestHarnessTeamJoinThroughAuthenticatedRPC(t *testing.T) {
	s, operator := testService(t)
	target := t.TempDir()
	invokeTeamRPC(t, s, operator, "teams.create", map[string]any{"id": "product", "target": target, "title": "Team", "text": "Shared brief"}, 200)
	for _, runtime := range []string{"hermes", "pi", "claude", "codex"} {
		invokeTeamRPC(t, s, operator, "sessions.register", map[string]any{"id": runtime, "target": target, "runtime": runtime, "mode": "manual"}, 200)
		token, err := s.Token(runtime)
		if err != nil {
			t.Fatal(err)
		}
		invokeTeamRPC(t, s, token, "teams.join", map[string]any{"id": "product"}, 400)
		invokeTeamRPC(t, s, operator, "teams.invite", map[string]any{"id": "product", "target": target, "to": runtime}, 200)
		invokeTeamRPC(t, s, token, "teams.join", map[string]any{"id": "product"}, 200)
		invokeTeamRPC(t, s, token, "teams.revoke", map[string]any{"id": "product", "target": target, "to": runtime}, 400)
	}
	for _, name := range []string{"teams.create", "teams.invite", "teams.get", "teams.join", "teams.leave", "teams.revoke"} {
		if !isTool(name) {
			t.Fatal("team method absent from MCP catalog", name)
		}
	}
}

func invokeTeamRPC(t *testing.T, s *core.Service, token, method string, params any, want int) {
	t.Helper()
	data, err := json.Marshal(map[string]any{"method": method, "params": params})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("POST", "/rpc", bytes.NewReader(data))
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	handler(s).ServeHTTP(response, request)
	if response.Code != want {
		t.Fatalf("%s returned %d want %d", method, response.Code, want)
	}
}
