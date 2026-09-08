// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"agentcommons/internal/core"
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A real child process stands in for the provider executable; no model is called.
func fixture(t *testing.T, name, body string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}
func TestCLIClaude(t *testing.T) {
	fixture(t, "claude", `input=$(cat)
case "$*" in *--permission-mode*dontAsk*--tools*Read,Glob,Grep*) ;; *) exit 4;; esac
case "$input" in *READ-ONLY*Peer\ envelope*) ;; *) exit 5;; esac
printf '%s' '{"session_id":"fixture","result":"inspected"}'
`)
	id, out, err := (CLI{}).Run(context.Background(), core.Session{Mode: "managed", Runtime: "claude", Target: t.TempDir()}, core.Delivery{Text: "inspect"})
	if err != nil || id != "fixture" || out != "inspected" {
		t.Fatalf("%q %q %v", id, out, err)
	}
}
func TestCLICodexResume(t *testing.T) {
	fixture(t, "codex", `cat >/dev/null
case "$*" in *read-only*resume*owned-session*) ;; *) exit 4;; esac
printf '%s\n' '{"type":"thread.started","thread_id":"owned-session"}' '{"type":"item.completed","item":{"type":"agent_message","text":"evidence"}}' '{"type":"turn.completed"}'
`)
	id, out, err := (CLI{}).Run(context.Background(), core.Session{Mode: "managed", Runtime: "codex", Target: t.TempDir(), RuntimeSessionID: "owned-session"}, core.Delivery{})
	if err != nil || id != "owned-session" || out != "evidence" {
		t.Fatalf("%q %q %v", id, out, err)
	}
}
func TestCLICancellation(t *testing.T) {
	fixture(t, "claude", "exec sleep 10\n")
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	_, _, err := (CLI{}).Run(ctx, core.Session{Mode: "managed", Runtime: "claude", Target: t.TempDir()}, core.Delivery{})
	if err == nil {
		t.Fatal("expected cancellation")
	}
}
func TestFailedAndIncompleteResults(t *testing.T) {
	for _, tc := range []struct{ runtime, raw string }{{"claude", `{"is_error":true,"result":"bad"}`}, {"claude", `{}`}, {"codex", `{"type":"turn.failed"}`}, {"codex", `{"type":"turn.completed"}`}} {
		if _, _, err := parse(tc.runtime, []byte(tc.raw)); err == nil {
			t.Fatal(tc)
		}
	}
}
func TestProjectDefinitions(t *testing.T) {
	target := t.TempDir()
	role := filepath.Join(target, ".claude", "agents")
	if err := os.MkdirAll(role, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(role, "reviewer.md")
	if err := os.WriteFile(path, []byte("PROJECT ADVERSARIAL REVIEW CHARTER"), 0600); err != nil {
		t.Fatal(err)
	}
	defs, err := Inventory(target)
	path, _ = filepath.EvalSymlinks(path)
	if err != nil || len(defs) != 1 || defs[0].Path != path || len(defs[0].Digest) != 64 {
		t.Fatalf("%+v %v", defs, err)
	}
	prompt, err := projectPrompt(target, "reviewer", "codex")
	if err != nil || !strings.Contains(prompt, "PROJECT ADVERSARIAL REVIEW CHARTER") || !strings.Contains(prompt, role) {
		t.Fatalf("%s %v", prompt, err)
	}
}
func TestMCPPrivateCredential(t *testing.T) {
	fixture(t, "claude", `cat >/dev/null
case "$*" in *secret-token*) exit 6;; esac
case "$*" in *--mcp-config*--token-file*) ;; *) exit 7;; esac
printf '%s' '{"session_id":"fixture","result":"tools configured"}'
`)
	state := t.TempDir()
	ctx := context.WithValue(context.Background(), credentialKey{}, "secret-token")
	_, _, err := (CLI{StateDir: state, Binary: "/bin/commons", Socket: "/tmp/socket"}).Run(ctx, core.Session{Mode: "managed", Runtime: "claude", Target: t.TempDir()}, core.Delivery{})
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(state, "runtime"))
	if err != nil || len(entries) != 0 {
		t.Fatal("credential not cleaned up", err)
	}
}

func TestLinkedSkillInventory(t *testing.T) {
	target := t.TempDir()
	assets := t.TempDir()
	if err := os.MkdirAll(filepath.Join(target, ".codex", "skills"), 0700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"SKILL.md": "Read references relative to here.", "reference.md": "support"} {
		if err := os.WriteFile(filepath.Join(assets, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(assets, filepath.Join(target, ".codex", "skills", "review")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(assets, filepath.Join(assets, "cycle")); err != nil {
		t.Fatal(err)
	}
	defs, err := Inventory(target)
	real, _ := filepath.EvalSymlinks(assets)
	if err != nil || len(defs) != 1 || defs[0].BaseDir != real {
		t.Fatalf("%+v %v", defs, err)
	}
}
func TestCodexDiscoveryFixture(t *testing.T) {
	fixture(t, "codex", `read init
printf '%s\n' '{"id":1,"result":{}}'
read notification
read listing
printf '{"id":2,"result":{"data":[{"id":"existing","cwd":"%s","name":"lead","status":{"type":"idle"}}],"nextCursor":null}}\n' "$PWD"
read remaining
`)
	target := t.TempDir()
	target, _ = filepath.EvalSymlinks(target)
	sessions, err := DiscoverCodex(context.Background(), target)
	if err != nil || len(sessions) != 1 || sessions[0].SessionID != "existing" {
		t.Fatalf("%+v %v", sessions, err)
	}
}
func TestOutputBound(t *testing.T) {
	var buffer boundedBuffer
	p := make([]byte, outputLimit+1)
	n, err := buffer.Write(p)
	if err != nil || n != len(p) || !buffer.exceeded || buffer.Len() != outputLimit {
		t.Fatal("output bound failed")
	}
}

// The descendant sleeps far longer than descendantDeath, so a surviving one is
// still a failure rather than a process that exited on its own. Both waits poll
// and return the moment the condition holds; the deadlines are paid only when
// the behaviour is actually wrong, never by a busy machine.
const descendantDeath = 20 * time.Second

func TestCancellationKillsDescendants(t *testing.T) {
	fixture(t, "claude", "sleep 120 &\necho $! > child.pid\nwait\n")
	target := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runErr := make(chan error, 1)
	go func() {
		_, _, err := (CLI{}).Run(ctx, core.Session{Mode: "managed", Runtime: "claude", Target: target}, core.Delivery{})
		runErr <- err
	}()
	pid := waitFixturePID(t, filepath.Join(target, "child.pid"))
	cancel()
	if err := <-runErr; err == nil {
		t.Fatal("expected canceled child")
	}
	deadline := time.After(descendantDeath)
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for syscall.Kill(pid, 0) == nil {
		select {
		case <-deadline:
			t.Fatalf("descendant %d survived cancellation", pid)
		case <-ticker.C:
		}
	}
}
func TestDiagnosticRedaction(t *testing.T) {
	fixture(t, "claude", "cat >/dev/null\nprintf '%s' 'provider unavailable bearer abc123 token=badvalue secret-token' >&2\nexit 3\n")
	ctx := context.WithValue(context.Background(), credentialKey{}, "secret-token")
	_, _, err := (CLI{}).Run(ctx, core.Session{Mode: "managed", Runtime: "claude", Target: t.TempDir()}, core.Delivery{})
	if err == nil || !strings.Contains(err.Error(), "provider unavailable") || strings.Contains(err.Error(), "abc123") || strings.Contains(err.Error(), "badvalue") || strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("diagnostic: %v", err)
	}
}
func TestProvenanceReceipt(t *testing.T) {
	fixture(t, "claude", "cat >/dev/null\nprintf '%s' '{\"session_id\":\"fixture\",\"result\":\"done\"}'\n")
	target := t.TempDir()
	state := t.TempDir()
	path := filepath.Join(target, ".claude", "agents")
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "reviewer.md"), []byte("local charter"), 0600); err != nil {
		t.Fatal(err)
	}
	_, _, err := (CLI{StateDir: state}).Run(context.Background(), core.Session{ID: "team/reviewer", Role: "reviewer", Mode: "managed", Runtime: "claude", Target: target}, core.Delivery{ID: "delivery-1"})
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(state, "receipts"))
	if err != nil || len(entries) != 1 {
		t.Fatal(entries, err)
	}
	receipt := filepath.Join(state, "receipts", entries[0].Name())
	raw, err := os.ReadFile(receipt)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"delivery-1", "team/reviewer", "ProjectPromptSHA256", "SelectedRolePath", "reviewer.md", "completed"} {
		if !strings.Contains(string(raw), want) {
			t.Fatal("missing receipt field", want)
		}
	}
	info, _ := os.Stat(receipt)
	if info.Mode().Perm() != 0600 {
		t.Fatal("receipt permissions")
	}
}

func TestProviderRefusalOnStdout(t *testing.T) {
	fixture(t, "claude", "cat >/dev/null\nprintf '%s' '{\"is_error\":true,\"errors\":[\"invalid_request [reasoning_extraction]\"]}'\nexit 1\n")
	_, _, err := (CLI{}).Run(context.Background(), core.Session{Mode: "managed", Runtime: "claude", Target: t.TempDir()}, core.Delivery{})
	if err == nil || !strings.Contains(err.Error(), "[reasoning_extraction]") {
		t.Fatalf("lost provider refusal: %v", err)
	}
}
