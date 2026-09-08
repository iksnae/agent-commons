// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestRoleCommandsFallBackToProjectManifest exercises actual command entry
// points (not just resolveConnectionConfigPath in isolation) with no --config
// and no AGENT_COMMONS_CONNECTION, from a directory nested under an enrolled
// project root, matching a real `init` layout: a real service, a real enroll
// and a real .agent-commons/project.json. Each command must resolve its own
// connection from the manifest rather than failing with "requires --config".
func TestRoleCommandsFallBackToProjectManifest(t *testing.T) {
	state := onboardingService(t)
	target := t.TempDir()
	target, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	enrolled := onboardingCommand(t, "enroll", "--state", state, "--target", target, "--name", "lead", "--role", "lead")
	var enrollment struct {
		Config string `json:"config"`
	}
	if err := json.Unmarshal(enrolled, &enrollment); err != nil {
		t.Fatal(err)
	}
	writeProjectManifest(t, target, projectDefaults{Version: 1, ProjectRoot: target, Team: "", Name: "lead", Role: "lead", Runtime: "claude", State: state})
	derived, err := resolveConnectionConfigPath("", target)
	if err != nil {
		t.Fatal(err)
	}
	if derived != enrollment.Config {
		t.Fatalf("manifest derivation %q disagrees with real enrollment %q", derived, enrollment.Config)
	}
	nested := filepath.Join(target, "a", "b")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENT_COMMONS_CONNECTION", "")
	t.Chdir(nested)

	t.Run("doctor", func(t *testing.T) {
		var out bytes.Buffer
		if err := run(context.Background(), []string{"doctor"}, nil, &out, io.Discard); err != nil {
			t.Fatalf("doctor did not resolve connection from manifest: %v (%s)", err, out.String())
		}
		var report healthReport
		if err := json.Unmarshal(out.Bytes(), &report); err != nil || !report.Ready {
			t.Fatalf("doctor manifest-resolved check unhealthy: %s", out.String())
		}
	})

	t.Run("connect-mcp", func(t *testing.T) {
		var out bytes.Buffer
		in := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)
		if err := run(context.Background(), []string{"connect-mcp"}, in, &out, io.Discard); err != nil {
			t.Fatalf("connect-mcp did not resolve connection from manifest: %v (%s)", err, out.String())
		}
		if !bytes.Contains(out.Bytes(), []byte(`"result"`)) {
			t.Fatalf("connect-mcp manifest-resolved call failed: %s", out.String())
		}
	})

	t.Run("console", func(t *testing.T) {
		var out bytes.Buffer
		if err := run(context.Background(), []string{"console", "--once"}, nil, &out, io.Discard); err != nil {
			t.Fatalf("console did not resolve connection from manifest: %v (%s)", err, out.String())
		}
	})

	t.Run("check-in", func(t *testing.T) {
		var out bytes.Buffer
		args := []string{"check-in", "--runtime", "claude", "--native-session", "manifest-native"}
		if err := run(context.Background(), args, nil, &out, io.Discard); err != nil {
			t.Fatalf("check-in did not resolve connection from manifest: %v (%s)", err, out.String())
		}
	})
}

// TestRoleCommandsWithoutManifestOrConfigNameTheRemedy proves the negative
// case reaches every wired command instead of a generic error: with neither
// --config, AGENT_COMMONS_CONNECTION, nor a manifest anywhere upward, each
// command must fail with the "no Agent Commons project found ... init"
// message rather than the old blanket "--config required".
func TestRoleCommandsWithoutManifestOrConfigNameTheRemedy(t *testing.T) {
	t.Setenv("AGENT_COMMONS_CONNECTION", "")
	t.Chdir(t.TempDir())
	cases := map[string][]string{
		"doctor":      {"doctor"},
		"connect-mcp": {"connect-mcp"},
		"console":     {"console", "--once"},
		"check-in":    {"check-in", "--runtime", "claude", "--native-session", "x"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			err := run(context.Background(), args, nil, &out, &errOut)
			if err == nil {
				t.Fatalf("%s accepted with no config, env or manifest", name)
			}
			combined := err.Error() + out.String() + errOut.String()
			if !bytesContainsAll(combined, "no Agent Commons project found", "agent-commons init") {
				t.Fatalf("%s did not name the failure/remedy: %v / %s / %s", name, err, out.String(), errOut.String())
			}
		})
	}
}

func bytesContainsAll(haystack string, needles ...string) bool {
	for _, needle := range needles {
		if !bytes.Contains([]byte(haystack), []byte(needle)) {
			return false
		}
	}
	return true
}
