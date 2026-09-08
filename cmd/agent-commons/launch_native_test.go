// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"agentcommons/internal/core"
)

// --init-only runs native launch hooks without starting a model conversation.
// Config, target and service state are disposable; no existing sessions resume.
func TestNativeClaudeLaunchHook(t *testing.T) {
	if os.Getenv("AGENT_COMMONS_CLAUDE_HOOK") != "1" {
		t.Skip("native Claude hook test is opt-in")
	}
	claude, err := exec.LookPath("claude")
	if err != nil {
		t.Fatal(err)
	}
	binary := os.Getenv("AGENT_COMMONS_TEST_BINARY")
	if !filepath.IsAbs(binary) {
		t.Fatal("absolute AGENT_COMMONS_TEST_BINARY required")
	}
	plugin, err := filepath.Abs("../../plugins/agent-commons")
	if err != nil {
		t.Fatal(err)
	}
	state, target := onboardingService(t), t.TempDir()
	data := onboardingCommand(t, "enroll", "--json", "--state", state, "--target", target, "--name", "lead", "--role", "lead")
	var enrollment struct {
		Config string `json:"config"`
	}
	if err = json.Unmarshal(data, &enrollment); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, claude, "--init-only", "--setting-sources", "", "--strict-mcp-config", "--mcp-config", `{"mcpServers":{}}`, "--plugin-dir", plugin)
	command.Dir = target
	command.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME"), "TMPDIR=" + os.Getenv("TMPDIR"), "CLAUDE_CONFIG_DIR=" + t.TempDir(), "AGENT_COMMONS_CONNECTION=" + enrollment.Config, "AGENT_COMMONS_BINARY=" + binary, "AGENT_COMMONS_CLAUDE_BINARY=" + claude, "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1"}
	command.WaitDelay = time.Second
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("native init-only failed: %v (%d output bytes; content withheld)", err, len(output))
	}
	connection, err := openProjectConnection(enrollment.Config)
	if err != nil {
		t.Fatal(err)
	}
	peers, err := rpcCall[[]core.Session](ctx, connection.client, "sessions.list", struct{}{})
	if err != nil || len(peers) != 1 || peers[0].Attachment.NativeID == "" || peers[0].Attachment.Runtime != "claude" {
		t.Fatal("native hook did not attach enrolled role", err)
	}
	inbox, err := rpcCall[struct {
		Messages []core.Delivery `json:"messages"`
	}](ctx, connection.client, "inbox.page", map[string]bool{"unreadOnly": true})
	if err != nil || len(inbox.Messages) != 2 {
		t.Fatal("launch acknowledged welcome messages", err)
	}
	t.Log("native Claude init-only loaded plugin hook, attached exact enrolled identity and preserved unread welcome messages; no model turn")
}
