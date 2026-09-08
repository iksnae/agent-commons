// SPDX-License-Identifier: MPL-2.0

package installation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const portableManifest = `{
  "$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
  "mcpServers": {
    "agent-commons": {
      "type": "stdio",
      "command": "agent-commons",
      "args": ["connect-mcp"]
    }
  }
}
`

const nativeManifest = `{
  "mcpServers": {
    "agent-commons": {
      "command": "agent-commons",
      "args": ["connect-mcp"]
    }
  }
}
`

// pluginBundleFixture adds the plugin MCP manifests the shipped bundle carries.
func pluginBundleFixture(t *testing.T) string {
	t.Helper()
	root := bundleFixture(t)
	if err := os.MkdirAll(filepath.Join(root, "plugins", "agent-commons"), 0700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"mcp.json": portableManifest, ".mcp.json": nativeManifest} {
		if err := os.WriteFile(filepath.Join(root, "plugins", "agent-commons", name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func serverCommand(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Servers map[string]struct {
			Command string `json:"command"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	server, ok := manifest.Servers["agent-commons"]
	if !ok {
		t.Fatal("manifest lost its agent-commons server", path)
	}
	return server.Command
}

// bundle install promises it changes no PATH, so the installed manifests must
// name the binary the same install placed rather than a bare command.
func TestInstallRewritesManifestCommandToInstalledBinary(t *testing.T) {
	bundle := pluginBundleFixture(t)
	target := filepath.Join(t.TempDir(), "installed")
	if err := Install(bundle, target); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(target, "agent-commons")
	for _, name := range []string{"mcp.json", ".mcp.json"} {
		path := filepath.Join(target, "plugins", "agent-commons", name)
		if command := serverCommand(t, path); command != binary {
			t.Fatal("installed manifest command not the installed binary", name, command)
		}
	}
	if command := serverCommand(t, filepath.Join(bundle, "plugins", "agent-commons", "mcp.json")); command != "agent-commons" {
		t.Fatal("install rewrote the source bundle", command)
	}
}

// The receipt must record the rewritten bytes; hashing the source instead makes
// verify reject the install's own output.
func TestVerifyAcceptsFreshlyInstalledManifests(t *testing.T) {
	target := filepath.Join(t.TempDir(), "installed")
	if err := Install(pluginBundleFixture(t), target); err != nil {
		t.Fatal(err)
	}
	if err := Verify(target); err != nil {
		t.Fatal("fresh installation failed its own verification", err)
	}
}

// Moving an installation leaves every recorded byte intact but strands the
// manifest command, which is the difference between intact and usable.
func TestVerifyRejectsStrandedManifestCommand(t *testing.T) {
	scratch := t.TempDir()
	target := filepath.Join(scratch, "installed")
	if err := Install(pluginBundleFixture(t), target); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(scratch, "moved")
	if err := os.Rename(target, moved); err != nil {
		t.Fatal(err)
	}
	err := Verify(moved)
	if err == nil {
		t.Fatal("verified an installation whose server command is missing")
	}
	if !strings.Contains(err.Error(), "command") {
		t.Fatal("unexpected failure reason", err)
	}
}

func TestManifestCommandMustBeAnAbsoluteExecutable(t *testing.T) {
	root := t.TempDir()
	write := func(name string, mode os.FileMode) string {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte("binary"), mode); err != nil {
			t.Fatal(err)
		}
		return path
	}
	manifest := func(command string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "mcp.json")
		body := strings.Replace(portableManifest, `"command": "agent-commons"`, `"command": `+quote(t, command), 1)
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	// Positive control: an absolute, executable command passes.
	if err := verifyManifestCommands(manifest(write("runnable", 0700))); err != nil {
		t.Fatal("rejected an absolute executable command", err)
	}
	for name, command := range map[string]string{
		"non-executable": write("inert", 0600),
		"missing":        filepath.Join(root, "absent"),
		"bare":           "agent-commons",
	} {
		t.Run(name, func(t *testing.T) {
			if err := verifyManifestCommands(manifest(command)); err == nil {
				t.Fatal("accepted an unusable server command", command)
			}
		})
	}
}

func quote(t *testing.T, value string) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
