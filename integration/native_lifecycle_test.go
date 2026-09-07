// SPDX-License-Identifier: MPL-2.0

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"agentcommons/internal/core"
)

// Opt-in: registers only a randomly named, temporary user-supervisor job.
// No provider sessions, existing state, permanent login units or root services.
func TestNativeSupervisorLifecycle(t *testing.T) {
	if os.Getenv("AGENT_COMMONS_NATIVE_SUPERVISOR") != "1" {
		t.Skip("native supervisor test is opt-in")
	}
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Fatal("unsupported native supervisor")
	}
	f := newNativeFixture(t)
	started := false
	registered := true
	t.Cleanup(func() {
		if started {
			if err := f.manager.stop(); err != nil {
				t.Error(err)
				return
			}
		}
		if registered {
			if err := f.manager.uninstall(); err != nil {
				t.Error(err)
			}
		}
	})
	// Mark before start: a partially successful registration still needs cleanup.
	started = true
	if err := f.manager.start(); err != nil {
		t.Fatal(err)
	}
	firstPID := f.waitReady(t, 0)
	_, err := f.call("sessions.register", core.Session{ID: "native-test", Target: f.state, Name: "lead", Role: "lead", Team: "test", Runtime: "manual", Mode: "manual"})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.manager.crash(); err != nil {
		t.Fatal(err)
	}
	f.waitReady(t, firstPID)
	assertNativeRegistry(t, f)
	if err = f.manager.stop(); err != nil {
		t.Fatal(err)
	}
	started = false
	f.waitStopped(t)
	if _, err = os.Stat(filepath.Join(f.state, "state.json")); err != nil {
		t.Fatal("stop lost state", err)
	}
	started = true
	if err = f.manager.start(); err != nil {
		t.Fatal(err)
	}
	f.waitReady(t, 0)
	assertNativeRegistry(t, f)
	if err = f.manager.stop(); err != nil {
		t.Fatal(err)
	}
	started = false
	f.waitStopped(t)
	if err = f.manager.uninstall(); err != nil {
		t.Fatal(err)
	}
	registered = false
	if err = os.Remove(f.manager.file); err != nil {
		t.Fatal(err)
	}
	if err = f.manager.verifyUninstalled(); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(f.state, "state.json")); err != nil {
		t.Fatal("uninstall lost state", err)
	}
	t.Log("native supervisor: start, SIGKILL recovery, durable registry, stop and transient uninstall passed")
}

func assertNativeRegistry(t *testing.T, f nativeFixture) {
	t.Helper()
	raw, err := f.call("sessions.list", struct{}{})
	if err != nil {
		t.Fatal(err)
	}
	var peers []core.Session
	if err = json.Unmarshal(raw, &peers); err != nil {
		t.Fatal(err)
	}
	if len(peers) != 1 || peers[0].ID != "native-test" {
		t.Fatal("registry did not survive native restart")
	}
}
