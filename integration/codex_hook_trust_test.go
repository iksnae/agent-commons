// SPDX-License-Identifier: MPL-2.0

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Discovery and trust metadata are distinct from actual hook execution.
func TestNativeCodexReportsLaunchHookTrust(t *testing.T) {
	if os.Getenv("AGENT_COMMONS_CODEX_HOOK") != "1" {
		t.Skip("native Codex hook test is opt-in")
	}
	configDir, target := t.TempDir(), t.TempDir()
	path := filepath.Join(configDir, "hooks.json")
	const hook = `{"hooks":{"SessionStart":[{"matcher":"^startup$","hooks":[{"type":"command","command":"printf AGENT_COMMONS_TRUST_FIXTURE","timeout":3}]}]}}`
	if err := os.WriteFile(path, []byte(hook), 0600); err != nil {
		t.Fatal(err)
	}
	request := codexFixture(t, configDir, target)
	raw := request("hooks/list", map[string]any{"cwds": []string{target}})
	var response struct {
		Data []struct {
			CWD    string            `json:"cwd"`
			Errors []json.RawMessage `json:"errors"`
			Hooks  []struct {
				SourcePath  string `json:"sourcePath"`
				EventName   string `json:"eventName"`
				TrustStatus string `json:"trustStatus"`
				CurrentHash string `json:"currentHash"`
				Enabled     bool   `json:"enabled"`
				IsManaged   bool   `json:"isManaged"`
			} `json:"hooks"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Data) != 1 || response.Data[0].CWD != target || len(response.Data[0].Errors) != 0 || len(response.Data[0].Hooks) != 1 {
		t.Fatal("native hook discovery did not return exactly the isolated launch hook")
	}
	found := response.Data[0].Hooks[0]
	canonicalPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	if found.SourcePath != canonicalPath || found.EventName != "sessionStart" || found.TrustStatus != "untrusted" || found.CurrentHash == "" || !found.Enabled || found.IsManaged {
		t.Fatalf("native hook metadata did not distinguish enabled from trusted: %+v", found)
	}
	t.Log("native Codex discovered the launch hook as enabled but untrusted, with a definition hash; no hook trust granted or model turn started")
}
