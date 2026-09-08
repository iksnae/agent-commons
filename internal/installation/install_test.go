// SPDX-License-Identifier: MPL-2.0

package installation

import (
	"os"
	"path/filepath"
	"testing"
)

// Boundary fixtures exercise the filesystem adapter, not the coordination core.
func bundleFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{"agent-commons", "LICENSE", "LICENSING.md", "INSTALL.md", "README.md", "source.tar.gz", "third-party-notices/go/LICENSE"} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fixture "+name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestInstallAndRecoverableRemoval(t *testing.T) {
	bundle := pluginBundleFixture(t)
	target := filepath.Join(t.TempDir(), "installed")
	if err := Install(bundle, target); err != nil {
		t.Fatal(err)
	}
	if err := Verify(target); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(target, "agent-commons"))
	if err != nil || info.Mode().Perm()&0100 == 0 {
		t.Fatal("binary not executable", err)
	}
	retained, err := Remove(target + string(filepath.Separator))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("installation path still exists")
	}
	// INSTALL.md documents recovery as moving the retained copy back to the
	// original path. The installed MCP command names that path, so only there
	// is the recovered installation usable rather than merely intact.
	if err = os.Rename(retained, target); err != nil {
		t.Fatal(err)
	}
	if err = Verify(target); err != nil {
		t.Fatal("recovered bundle not usable at its original path", err)
	}
}

func TestInstallRefusesExistingDestination(t *testing.T) {
	target := t.TempDir()
	if err := Install(bundleFixture(t), target); err == nil {
		t.Fatal("overwrote existing destination")
	}
}

func TestRemovalRefusesModifiedOrAdditionalFiles(t *testing.T) {
	for _, name := range []string{"README.md", "user-notes.txt"} {
		t.Run(name, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "installed")
			if err := Install(bundleFixture(t), target); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(target, name), []byte("keep me"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := Remove(target); err == nil {
				t.Fatal("removed changed installation")
			}
			data, err := os.ReadFile(filepath.Join(target, name))
			if err != nil || string(data) != "keep me" {
				t.Fatal("user data changed", err)
			}
		})
	}
}

func TestInstallRejectsSymlinkPayloadBeforeWriting(t *testing.T) {
	bundle := bundleFixture(t)
	if err := os.Remove(filepath.Join(bundle, "LICENSE")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("README.md", filepath.Join(bundle, "LICENSE")); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "installed")
	if err := Install(bundle, target); err == nil {
		t.Fatal("symlink accepted")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("invalid bundle created destination")
	}
}
