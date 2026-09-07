// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"agentcommons/internal/core"
)

func TestNativePiPackageChecksInExactSession(t *testing.T) {
	if os.Getenv("AGENT_COMMONS_PI_HOOK") != "1" {
		t.Skip("native Pi package test is opt-in; no model turn")
	}
	pi, err := exec.LookPath("pi")
	if err != nil {
		t.Fatal(err)
	}
	binary := os.Getenv("AGENT_COMMONS_TEST_BINARY")
	if !filepath.IsAbs(binary) {
		t.Fatal("absolute test binary required")
	}
	plugin, err := filepath.Abs("../../plugins/agent-commons")
	if err != nil {
		t.Fatal(err)
	}
	args, connection := codexPrepareFixture(t)
	home := t.TempDir()
	f := piFixture{binary: pi, directory: connection.config.Target, env: []string{
		"PATH=" + os.Getenv("PATH"), "HOME=" + home, "PI_CODING_AGENT_DIR=" + filepath.Join(home, "pi"),
		"PI_OFFLINE=1", "PI_TELEMETRY=0", "AGENT_COMMONS_CONNECTION=" + args[1], "AGENT_COMMONS_BINARY=" + binary,
	}}
	f.run(t, "install", plugin)
	// Seed only a native-format header: Pi otherwise defers creating a new
	// transcript until an assistant message. No model output is fabricated.
	transcript := filepath.Join(home, "root.jsonl")
	header, err := json.Marshal(map[string]any{"type": "session", "version": 3, "id": "a196ca83-c293-43d8-995a-7b7f6089ca21", "timestamp": "2026-09-07T00:00:00Z", "cwd": connection.config.Target})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(transcript, append(header, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	responses := f.inspectSession(t, "--session", transcript)
	var state struct {
		SessionID, SessionFile string
		IsStreaming            bool
	}
	if err = json.Unmarshal(responses["get_state"], &state); err != nil {
		t.Fatal(err)
	}
	if state.SessionID == "" || state.IsStreaming {
		t.Fatal("no idle native session identity")
	}
	if !strings.Contains(string(responses["get_messages"]), "Agent Commons checked in") {
		t.Fatal("native startup guidance missing")
	}
	peers, err := rpcCall[[]core.Session](context.Background(), connection.client, "sessions.list", struct{}{})
	if err != nil || len(peers) != 1 || peers[0].Attachment.NativeID != state.SessionID || peers[0].Attachment.Runtime != "pi" {
		t.Fatal("native Pi attachment mismatch", err)
	}
	var inbox struct{ Messages []core.Delivery }
	inbox, err = rpcCall[struct{ Messages []core.Delivery }](context.Background(), connection.client, "inbox.page", map[string]bool{"unreadOnly": true})
	if err != nil || len(inbox.Messages) != 2 {
		t.Fatal("startup acknowledged welcome messages", err)
	}
	if !filepath.IsAbs(state.SessionFile) {
		t.Fatal("native session not persisted")
	}
	if _, err = os.Stat(state.SessionFile); err != nil {
		t.Fatal("native transcript absent", err)
	}
	resumed := f.inspectSession(t, "--session", state.SessionFile)
	var resumedState struct{ SessionID string }
	if err = json.Unmarshal(resumed["get_state"], &resumedState); err != nil || resumedState.SessionID != state.SessionID {
		t.Fatal("exact session resume changed identity", err)
	}
	if strings.Count(string(resumed["get_messages"]), "Agent Commons checked in") != strings.Count(string(responses["get_messages"]), "Agent Commons checked in")+1 {
		t.Fatal("resume reused old guidance without a successful new check-in")
	}
	forked := f.inspectSession(t, "--fork", state.SessionFile)
	if !strings.Contains(string(forked["get_messages"]), "Agent Commons launch check-in failed") {
		t.Fatal("native fork did not report rejected check-in")
	}
	peers, err = rpcCall[[]core.Session](context.Background(), connection.client, "sessions.list", struct{}{})
	if err != nil || len(peers) != 1 || peers[0].Attachment.NativeID != state.SessionID {
		t.Fatal("fork changed role attachment", err)
	}
	f.run(t, "remove", plugin)
	after := f.inspectSession(t)
	if strings.Contains(string(after["get_messages"]), "Agent Commons") {
		t.Fatal("removed package still active")
	}
	t.Log("Pi local installer loaded shared package, checked in exact seeded native root, preserved unread onboarding, resumed its transcript and removed package; no model turn or automatic fresh-session persistence proof")
}
