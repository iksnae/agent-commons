// SPDX-License-Identifier: MPL-2.0

package integration

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeCodexInstallsBundledPlugin(t *testing.T) {
	if os.Getenv("AGENT_COMMONS_CODEX_HOOK") != "1" {
		t.Skip("native Codex plugin test is opt-in")
	}
	marketplace := codexPluginMarketplace(t)
	configDir := t.TempDir()
	target := t.TempDir()
	request := codexFixture(t, configDir, target)
	params := map[string]string{"marketplacePath": marketplace, "pluginName": "agent-commons"}
	request("plugin/install", params)
	raw := request("plugin/read", params)
	var result struct {
		Plugin struct {
			MCPServers []string `json:"mcpServers"`
			Skills     []struct {
				Name string `json:"name"`
				Path string `json:"path"`
			} `json:"skills"`
			Hooks   []json.RawMessage `json:"hooks"`
			Summary struct {
				ID        string `json:"id"`
				Installed bool   `json:"installed"`
				Enabled   bool   `json:"enabled"`
			} `json:"summary"`
		} `json:"plugin"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	plugin := result.Plugin
	if !plugin.Summary.Installed || !plugin.Summary.Enabled {
		t.Fatal("native installation did not enable the plugin")
	}
	if len(plugin.MCPServers) != 1 || plugin.MCPServers[0] != "agent-commons" {
		t.Fatalf("native loader did not expose the MCP server: %v", plugin.MCPServers)
	}
	if len(plugin.Skills) != 1 || plugin.Skills[0].Name != "agent-commons:agent-commons" || !filepath.IsAbs(plugin.Skills[0].Path) {
		t.Fatalf("native loader did not expose the bundled skill: %+v", plugin.Skills)
	}
	if len(plugin.Hooks) != 0 {
		t.Fatal("Codex unexpectedly loaded the Claude-only launch hook")
	}
	installedRoot := codexInstalledSkillRoot(t, request, target)
	installedRoot, err := filepath.EvalSymlinks(installedRoot)
	if err != nil {
		t.Fatal(err)
	}
	canonicalConfig, err := filepath.EvalSymlinks(configDir)
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(canonicalConfig, installedRoot)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		t.Fatalf("native plugin path outside disposable Codex home: %s", installedRoot)
	}
	assertCodexPluginPayload(t, installedRoot)
	if plugin.Summary.ID == "" {
		t.Fatal("native installation omitted plugin ID")
	}
	request("plugin/uninstall", map[string]string{"pluginId": plugin.Summary.ID})
	if err := json.Unmarshal(request("plugin/read", params), &result); err != nil {
		t.Fatal(err)
	}
	if result.Plugin.Summary.Installed {
		t.Fatal("native removal retained installed status")
	}
	if _, err := os.Stat(installedRoot); !os.IsNotExist(err) {
		t.Fatal("native removal retained plugin cache", err)
	}
	t.Log("native Codex installed the complete bundle, exposed MCP/skill metadata, excluded the Claude hook and removed its cache; no model turn or MCP process started")
}

func assertCodexPluginPayload(t *testing.T, installedRoot string) {
	t.Helper()
	source := os.DirFS("../plugins/agent-commons")
	err := fs.WalkDir(source, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		expected, err := fs.ReadFile(source, path)
		if err != nil {
			return err
		}
		actual, err := os.ReadFile(filepath.Join(installedRoot, path))
		if err != nil {
			return err
		}
		if !bytes.Equal(actual, expected) {
			t.Errorf("native installation changed bundled file %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func codexPluginMarketplace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	plugin, err := filepath.Abs("../plugins/agent-commons")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.CopyFS(filepath.Join(root, "plugins", "agent-commons"), os.DirFS(plugin)); err != nil {
		t.Fatal(err)
	}
	metadata := filepath.Join(root, ".agents", "plugins")
	if err = os.MkdirAll(metadata, 0700); err != nil {
		t.Fatal(err)
	}
	// This disposable catalog installs only the copied product bundle.
	const catalog = `{"name":"agent-commons-test","plugins":[{"name":"agent-commons","source":{"source":"local","path":"./plugins/agent-commons"},"policy":{"installation":"AVAILABLE","authentication":"ON_INSTALL"},"category":"Productivity"}]}`
	path := filepath.Join(metadata, "marketplace.json")
	if err = os.WriteFile(path, []byte(catalog), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
