// SPDX-License-Identifier: MPL-2.0

package integration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"agentcommons/internal/supervision"
	"agentcommons/internal/transport"
)

type nativeFixture struct {
	manager nativeManager
	state   string
}

func newNativeFixture(t *testing.T) nativeFixture {
	t.Helper()
	root, err := os.MkdirTemp("", "ac-native-")
	if err != nil {
		t.Fatal(err)
	}
	// Preserve the fixture on failure so an operator can inspect/remove its job.
	t.Cleanup(func() {
		if !t.Failed() {
			os.RemoveAll(root)
		} else {
			t.Logf("retained fixture: %s", root)
		}
	})
	data, err := os.ReadFile(os.Getenv("AGENT_COMMONS_TEST_BINARY"))
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(root, "agent-commons")
	if err = os.WriteFile(binary, data, 0700); err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(root, "state")
	p, err := supervision.Render(supervision.Options{Platform: runtime.GOOS, Binary: binary, State: state, SearchPath: "/usr/bin:/bin"})
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, p.Filename)
	if err = os.WriteFile(file, []byte(p.Content), 0600); err != nil {
		t.Fatal(err)
	}
	return nativeFixture{nativeManager{p.Label, file}, state}
}

func (f nativeFixture) call(method string, params any) (json.RawMessage, error) {
	token, err := os.ReadFile(filepath.Join(f.state, "operator.token"))
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return transport.Call(ctx, filepath.Join(f.state, "service.sock"), strings.TrimSpace(string(token)), method, body)
}

func (f nativeFixture) waitReady(t *testing.T, previousPID int) int {
	t.Helper()
	deadline := time.Now().Add(40 * time.Second)
	for time.Now().Before(deadline) {
		pid, err := f.manager.pid()
		if err == nil && pid != previousPID {
			if _, err = f.call("sessions.list", struct{}{}); err == nil {
				return pid
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("supervised service did not become ready with a new PID")
	return 0
}

func (f nativeFixture) waitStopped(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		_, err := os.Lstat(filepath.Join(f.state, "service.sock"))
		if os.IsNotExist(err) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("service socket remained after stop")
}
