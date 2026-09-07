// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"agentcommons/internal/core"
)

func TestTeamRevocationStopsManagedProcessGroup(t *testing.T) {
	fixture(t, "codex", "cat >/dev/null\necho $$ > runner.pid\nsleep 20 &\necho $! > descendant.pid\nwait\n")
	s, target, task := supervisorFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, s, CLI{}, 5*time.Millisecond) }()
	defer func() { cancel(); awaitSupervisor(t, done) }()
	parent := waitFixturePID(t, filepath.Join(target, "runner.pid"))
	child := waitFixturePID(t, filepath.Join(target, "descendant.pid"))
	supervisorCall(t, s, "operator", "teams.revoke", map[string]any{"id": "product", "target": target, "to": "builder"})
	deadline := time.After(3 * time.Second)
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		got := supervisorCall(t, s, "operator", "tasks.get", map[string]any{"id": task.ID}).(core.Task)
		if got.Status == "failed" && syscall.Kill(parent, 0) != nil && syscall.Kill(child, 0) != nil {
			if got.Output != "" || got.Revision != 0 {
				t.Fatal("revoked process submitted work")
			}
			return
		}
		select {
		case <-deadline:
			t.Fatal("revocation did not stop owned process group and fail task")
		case <-ticker.C:
		}
	}
}

func waitFixturePID(t *testing.T, path string) int {
	t.Helper()
	deadline := time.After(3 * time.Second)
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		data, err := os.ReadFile(path)
		if err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && pid > 1 {
				return pid
			}
		}
		select {
		case <-deadline:
			t.Fatal("fixture process did not start")
		case <-ticker.C:
		}
	}
}
