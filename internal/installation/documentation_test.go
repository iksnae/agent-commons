// SPDX-License-Identifier: MPL-2.0

package installation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallPreservesGuidesAndPluginWithoutExecutingThem(t *testing.T) {
	bundle := bundleFixture(t)
	paths := []string{"PRODUCTION.md", "START-HERE.md", "docs/README.md", "docs/assets/agent-commons-hero.png", "docs/assets/agent-commons-icon.png", "integrations/ONBOARDING.md", "gates/codex-launch.md", "plugins/agent-commons/scripts/check-in.sh"}
	for _, path := range paths {
		name := filepath.Join(bundle, path)
		if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte("must not execute"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	target := filepath.Join(t.TempDir(), "installed")
	if err := Install(bundle, target); err != nil {
		t.Fatal(err)
	}
	if err := Verify(target); err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		data, err := os.ReadFile(filepath.Join(target, path))
		if err != nil || string(data) != "must not execute" {
			t.Fatal("lost guide/plugin payload", path, err)
		}
	}
	info, err := os.Stat(filepath.Join(target, "plugins/agent-commons/scripts/check-in.sh"))
	if err != nil || info.Mode().Perm() != 0700 {
		t.Fatal("plugin executable mode lost", err)
	}
}

func TestDocumentationPayloadRejectsUnrelatedAndEscapingPaths(t *testing.T) {
	for _, path := range []string{"notes.md", "docs/run.sh", "docs/assets/other.png", "docs/../operator.token", "plugins/other/run.sh", "plugins/agent-commons/../../outside", "/docs/README.md"} {
		if payloadPath(path) {
			t.Error("accepted unrelated payload", path)
		}
	}
}
