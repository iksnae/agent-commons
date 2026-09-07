// SPDX-License-Identifier: MPL-2.0

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"agentcommons/internal/core"
	"agentcommons/internal/supervision"
)

// Persistent registration is restricted to explicitly opted-in disposable CI.
// This proves the CLI path, not behavior across login or machine reboot.
func TestNativeServiceCLI(t *testing.T) {
	if os.Getenv("AGENT_COMMONS_NATIVE_SERVICE_CLI") != "1" || os.Getenv("GITHUB_ACTIONS") != "true" {
		t.Skip("persistent service CLI test requires disposable CI opt-in")
	}
	f := newNativeFixture(t)
	// Replace only the file created by this fixture with a CLI-owned installation.
	if err := os.Remove(f.manager.file); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(filepath.Dir(f.manager.file), "agent-commons")
	output, err := nativeCommand(binary, "service", "install", "--directory", filepath.Dir(f.manager.file), "--binary", binary, "--state", f.state, "--path", "/usr/bin:/bin")
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]string
	if err = json.Unmarshal([]byte(output), &result); err != nil || result["file"] != f.manager.file {
		t.Fatal("unexpected installed path", err, output)
	}
	cli := serviceCLI{binary: binary, file: f.manager.file}
	removed := false
	t.Cleanup(func() {
		if !removed {
			// Cleanup is scoped to this random job. Keep files if either action fails.
			if _, err := cli.run("stop"); err != nil {
				t.Error("cleanup could not confirm stop; fixture retained", err)
				return
			}
			f.waitStopped(t)
			if _, err := cli.run("remove", "--confirm-stopped"); err != nil {
				t.Error("cleanup failed; fixture retained", err)
			}
		}
	})
	cli.must(t, "enable")
	assertPersistentServiceLinks(t, f.manager, true)
	cli.must(t, "start")
	pid := f.waitReady(t, 0)
	cli.must(t, "status")
	if _, err = f.call("sessions.register", core.Session{ID: "native-test", Target: f.state, Name: "lead", Role: "lead", Team: "test", Runtime: "manual", Mode: "manual"}); err != nil {
		t.Fatal(err)
	}
	if err = f.manager.crash(); err != nil {
		t.Fatal(err)
	}
	f.waitReady(t, pid)
	assertNativeRegistry(t, f)
	cli.must(t, "stop")
	f.waitStopped(t)
	cli.must(t, "start")
	f.waitReady(t, 0)
	assertNativeRegistry(t, f)
	cli.must(t, "stop")
	f.waitStopped(t)
	output = cli.must(t, "remove", "--confirm-stopped")
	removed = true
	if err = json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatal(err)
	}
	retained := filepath.Join(result["retained"], filepath.Base(f.manager.file))
	if _, err = supervision.ReadInstalled(retained); err != nil {
		t.Fatal("removed config not recoverable", err)
	}
	if err = f.manager.verifyUninstalled(); err != nil {
		t.Fatal(err)
	}
	assertPersistentServiceLinks(t, f.manager, false)
	if _, err = os.Stat(filepath.Join(f.state, "state.json")); err != nil {
		t.Fatal("removed service state", err)
	}
	t.Log("service CLI: install, enable, start, status, crash recovery, restart, stop and recoverable removal passed")
}

type serviceCLI struct{ binary, file string }

func (c serviceCLI) run(operation string, extra ...string) (string, error) {
	args := append([]string{"service", operation, "--file", c.file}, extra...)
	return nativeCommand(c.binary, args...)
}
func (c serviceCLI) must(t *testing.T, operation string, extra ...string) string {
	t.Helper()
	output, err := c.run(operation, extra...)
	if err != nil {
		t.Fatal(err)
	}
	return output
}
