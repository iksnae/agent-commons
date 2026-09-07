// SPDX-License-Identifier: MPL-2.0

package installation

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallReservesReceiptEntry(t *testing.T) {
	bundle := bundleFixture(t)
	// Fixture has seven files, two directories, and the root entry.
	for i := 10; i < maxEntries-1; i++ {
		path := filepath.Join(bundle, "third-party-notices", fmt.Sprintf("notice-%04d", i))
		if err := os.WriteFile(path, nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	target := filepath.Join(t.TempDir(), "installed")
	if err := Install(bundle, target); err != nil {
		t.Fatal(err)
	}
	if err := Verify(target); err != nil {
		t.Fatal("maximum accepted bundle cannot be verified", err)
	}
	if err := os.WriteFile(filepath.Join(bundle, "third-party-notices", "extra"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	refused := filepath.Join(t.TempDir(), "refused")
	if err := Install(bundle, refused); err == nil {
		t.Fatal("accepted bundle without room for receipt")
	}
	if _, err := os.Stat(refused); !os.IsNotExist(err) {
		t.Fatal("oversized bundle created destination")
	}
}

func TestInstallRejectsSymlinkParentBeforeWriting(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "linked")
	actual := t.TempDir()
	if err := os.Symlink(actual, parent); err != nil {
		t.Fatal(err)
	}
	if err := Install(bundleFixture(t), filepath.Join(parent, "installed")); err == nil {
		t.Fatal("accepted symlink parent")
	}
	if _, err := os.Stat(filepath.Join(actual, "installed")); !os.IsNotExist(err) {
		t.Fatal("rejected parent left installation")
	}
}
