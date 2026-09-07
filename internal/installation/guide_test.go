// SPDX-License-Identifier: MPL-2.0

package installation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallCarriesWakeMaintenanceGuide(t *testing.T) {
	bundle := bundleFixture(t)
	const name = "WAKE-MAINTENANCE.md"
	if err := os.WriteFile(filepath.Join(bundle, name), []byte("maintenance guide"), 0600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "installed")
	if err := Install(bundle, target); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(target, name))
	if err != nil || string(data) != "maintenance guide" {
		t.Fatal("maintenance guide not installed", err)
	}
}
