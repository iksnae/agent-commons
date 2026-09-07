// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInventoryIncludesPiHermesAndSharedProjectSkills(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{".pi/skills/review/SKILL.md", ".pi/prompts/review.md", ".hermes/skills/review/SKILL.md", ".agents/skills/review/SKILL.md"} {
		path = filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("Project-owned review instructions"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	definitions, err := Inventory(root)
	if err != nil || len(definitions) != 4 {
		t.Fatal("missing project sources", len(definitions), err)
	}
	found := map[string]bool{}
	for _, definition := range definitions {
		found[definition.Runtime+"/"+definition.Kind] = true
		if definition.Name != "review" || definition.Digest == "" || definition.BaseDir != filepath.Dir(definition.Path) {
			t.Fatal("source provenance lost")
		}
	}
	for _, key := range []string{"pi/skills", "pi/commands", "hermes/skills", "shared/skills"} {
		if !found[key] {
			t.Fatal("missing source", key)
		}
	}
}
