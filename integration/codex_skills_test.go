// SPDX-License-Identifier: MPL-2.0

package integration

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func codexInstalledSkillRoot(t *testing.T, request func(string, any) json.RawMessage, target string) string {
	t.Helper()
	raw := request("skills/list", map[string]any{"cwds": []string{target}, "forceReload": true})
	var response struct {
		Data []struct {
			Skills []struct {
				Name string `json:"name"`
				Path string `json:"path"`
			} `json:"skills"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		t.Fatal(err)
	}
	for _, entry := range response.Data {
		for _, skill := range entry.Skills {
			if skill.Name == "agent-commons:agent-commons" {
				if !filepath.IsAbs(skill.Path) {
					t.Fatal("installed skill has no absolute path")
				}
				return filepath.Dir(filepath.Dir(filepath.Dir(skill.Path)))
			}
		}
	}
	t.Fatal("installed plugin skill not available in target")
	return ""
}
