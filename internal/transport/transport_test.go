// SPDX-License-Identifier: MPL-2.0

package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agentcommons/internal/core"
)

func testService(t *testing.T) (*core.Service, string) {
	t.Helper()
	s, err := core.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	token, err := s.Token("operator")
	if err != nil {
		t.Fatal(err)
	}
	return s, token
}
func TestHTTPAuthenticationAndFraming(t *testing.T) {
	s, token := testService(t)
	cases := []struct {
		token, body string
		code        int
	}{
		{"", `{"method":"sessions.list","params":{}}`, 401},
		{"wrong", `{"method":"sessions.list","params":{}}`, 401},
		{token, `{"method":"sessions.list","params":{}}`, 200},
		{token, `{"method":"sessions.list","params":{}} {}`, 400},
		{token, `{"method":"sessions.list","actor":"operator","params":{}}`, 400},
		{token, `{"method":"not.a.method","params":{}}`, 400},
	}
	for _, tc := range cases {
		req := httptest.NewRequest("POST", "/rpc", strings.NewReader(tc.body))
		if tc.token != "" {
			req.Header.Set("Authorization", "Bearer "+tc.token)
		}
		w := httptest.NewRecorder()
		handler(s).ServeHTTP(w, req)
		if w.Code != tc.code {
			t.Fatalf("status %d want %d: %s", w.Code, tc.code, w.Body.String())
		}
	}
}
func TestSocketRPCAndMCP(t *testing.T) {
	s, token := testService(t)
	// Short directory avoids platform Unix socket pathname limits.
	dir, err := os.MkdirTemp("", "ac-rpc-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	socket := filepath.Join(dir, "s")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, s, socket) }()
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, err = Call(ctx, socket, token, "sessions.list", json.RawMessage(`{}`)); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	info, err := os.Stat(socket)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("socket permissions: %v %v", info, err)
	}
	if _, err = Call(ctx, socket, "bad", "sessions.list", json.RawMessage(`{}`)); err == nil {
		t.Fatal("invalid token accepted")
	}
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"sessions.list","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"sessions.register","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"nope"}`,
		`broken`,
	}, "\n")
	var out bytes.Buffer
	if err = MCP(ctx, strings.NewReader(input), &out, socket, token); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 6 {
		t.Fatalf("notification emitted output: %s", out.String())
	}
	for i, line := range lines {
		var r rpcResponse
		if err = json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatal(err)
		}
		if i < 3 && r.Error != nil {
			t.Fatalf("unexpected RPC error: %s", line)
		}
		if i >= 3 && r.Error == nil {
			t.Fatalf("missing RPC error: %s", line)
		}
	}
	out.Reset()
	if err = MCP(ctx, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"sessions.list"}}`), &out, socket, "bad"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"isError":true`) {
		t.Fatal("MCP hid authentication error")
	}
}

// Operator evidence must not reach an agent through the DATA surface either.
// Keeping the tool surface clean is only half of it: a reinstated identity is
// listed to every peer scoped to its target, and it carries the ledger of why
// it was withdrawn and why it came back. This asserts on the bytes the peer's
// own credential retrieves over the wire.
func TestRetirementEvidenceNeverCrossesTheAgentBoundary(t *testing.T) {
	s, operator := testService(t)
	target, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	call := func(token, body string) string {
		t.Helper()
		req := httptest.NewRequest("POST", "/rpc", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		handler(s).ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("status %d: %s", w.Code, w.Body.String())
		}
		return w.Body.String()
	}

	const withdrawal = "OPERATOR-ONLY-WITHDRAWAL-PROSE"
	const restoration = "OPERATOR-ONLY-RESTORATION-PROSE"
	for _, id := range []string{"peer-one", "peer-two"} {
		if _, err := s.Call("operator", "sessions.register", []byte(`{"id":"`+id+`","target":"`+target+`","runtime":"manual","mode":"manual","policy":"coordination"}`)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Call("operator", "sessions.retire", []byte(`{"id":"peer-two","evidence":"`+withdrawal+`"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Call("operator", "sessions.reinstate", []byte(`{"id":"peer-two","evidence":"`+restoration+`"}`)); err != nil {
		t.Fatal(err)
	}

	peer, err := s.Token("peer-one")
	if err != nil {
		t.Fatal(err)
	}
	seen := call(peer, `{"method":"sessions.list","params":{}}`)
	if !strings.Contains(seen, "peer-two") {
		t.Fatalf("fixture is vacuous: the reinstated identity is not in the peer's listing: %s", seen)
	}
	for _, secret := range []string{withdrawal, restoration, "retirements", "retiredAt", "retiredReason"} {
		if strings.Contains(seen, secret) {
			t.Fatalf("peer-facing sessions.list carries %q: %s", secret, seen)
		}
	}
	// The operator, who owns the evidence, still receives it over the same wire.
	held := call(operator, `{"method":"sessions.list","params":{"includeRetired":true}}`)
	if !strings.Contains(held, withdrawal) || !strings.Contains(held, restoration) {
		t.Fatalf("operator lost the retirement ledger: %s", held)
	}
	// A peer cannot ask for it either.
	req := httptest.NewRequest("POST", "/rpc", strings.NewReader(`{"method":"sessions.list","params":{"includeRetired":true}}`))
	req.Header.Set("Authorization", "Bearer "+peer)
	w := httptest.NewRecorder()
	handler(s).ServeHTTP(w, req)
	if w.Code == 200 {
		t.Fatalf("a peer set includeRetired: %s", w.Body.String())
	}
}

func TestToolSurfaceHasNoAdministrativeEscalation(t *testing.T) {
	for _, name := range []string{"sessions.register", "messages.retry", "sessions.retire", "sessions.reinstate", "Token"} {
		if isTool(name) {
			t.Fatalf("admin tool exposed: %s", name)
		}
	}
	for _, def := range toolDefinitions() {
		if def.Description == "" || def.InputSchema["additionalProperties"] != false {
			t.Fatalf("incomplete definition %s", def.Name)
		}
		if def.Name == "tasks.review" {
			found := false
			for _, name := range def.InputSchema["required"].([]string) {
				if name == "expectedRevision" {
					found = true
				}
			}
			if !found {
				t.Fatal("review permits an unbound revision")
			}
		}
	}
}

func TestAgentCredentialCannotRegisterOrForgeSender(t *testing.T) {
	s, _ := testService(t)
	target := t.TempDir()
	for _, id := range []string{"alice", "bob"} {
		b, _ := json.Marshal(core.Session{ID: id, Target: target, Team: "team", Role: "builder", Runtime: "manual", Mode: "manual"})
		if _, err := s.Call("operator", "sessions.register", b); err != nil {
			t.Fatal(err)
		}
	}
	token, err := s.Token("alice")
	if err != nil {
		t.Fatal(err)
	}
	for i, body := range []string{`{"method":"sessions.register","params":{"id":"intruder"}}`, `{"method":"messages.send","params":{"to":"bob","text":"hello","idempotencyKey":"forged","from":"operator"}}`} {
		req := httptest.NewRequest("POST", "/rpc", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		handler(s).ServeHTTP(w, req)
		if i == 0 && w.Code == 200 {
			t.Fatalf("privilege escalation accepted: %s", w.Body.String())
		}
		if i == 1 && w.Code == 200 && !strings.Contains(w.Body.String(), `"from":"alice"`) {
			t.Fatalf("forged sender: %s", w.Body.String())
		}
	}
}

func TestRPCDiagnosticsOmitCredentialsAndPayload(t *testing.T) {
	s, token := testService(t)
	target := t.TempDir()
	raw, _ := json.Marshal(core.Session{ID: "diagnostic", Target: target, Runtime: "manual", Mode: "manual"})
	if _, err := s.Call("operator", "sessions.register", raw); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(previous)
	req := httptest.NewRequest("POST", "/rpc", strings.NewReader(`{"method":"messages.send","params":{"to":"diagnostic","text":"PRIVATE-PAYLOAD","idempotencyKey":"PRIVATE-KEY"}}`))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler(s).ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if !strings.Contains(logs.String(), `actor="operator" method="messages.send"`) {
		t.Fatalf("missing operational evidence: %s", logs.String())
	}
	for _, secret := range []string{token, "PRIVATE-PAYLOAD", "PRIVATE-KEY", "diagnostic"} {
		if strings.Contains(logs.String(), secret) {
			t.Fatal("diagnostics leaked request or credential")
		}
	}
}
