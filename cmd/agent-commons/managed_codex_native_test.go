// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"agentcommons/internal/core"
)

func TestNativeManagedCodexPreparationRecoversSameRoot(t *testing.T) {
	if os.Getenv("AGENT_COMMONS_CODEX_HOOK") != "1" {
		t.Skip("opt-in native Codex preparation; disposable profile, no model turn")
	}
	home, state, target := t.TempDir(), t.TempDir(), t.TempDir()
	for _, dir := range []string{home, state} {
		if err := os.Chmod(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	target, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_HOME", home)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	session := core.Session{ID: "disposable-role", Target: target, Runtime: "codex", Mode: "managed"}
	first, err := prepareManagedCodex(ctx, state, session, startCodexBootstrap)
	if err != nil || first == "" {
		t.Fatal("native preparation failed", err)
	}
	second, err := prepareManagedCodex(ctx, state, session, startCodexBootstrap)
	if err != nil || second != first {
		t.Fatal("native preparation recovery changed identity", err)
	}
	t.Log("fresh native preparation and recovery used the same durable root; no model turn")
}
