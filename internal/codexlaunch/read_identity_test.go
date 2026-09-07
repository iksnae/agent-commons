// SPDX-License-Identifier: MPL-2.0

package codexlaunch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadReadyRejectsConflictingOrUnsupportedEvidence(t *testing.T) {
	for _, mutation := range []struct{ name, from, to string }{
		{"reservation.json", `"version":1`, `"version":2`},
		{"reservation.json", `"version":1`, `"version":1,"unexpected":true`},
		{"created.json", savedThread, "00000000-0000-0000-0000-000000000002"},
		{"ready.json", savedThread, "invalid-thread"},
	} {
		t.Run(mutation.name+mutation.to, func(t *testing.T) {
			path, scope := readyJournalFixture(t)
			file := filepath.Join(path, mutation.name)
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			changed := strings.Replace(string(data), mutation.from, mutation.to, 1)
			if changed == string(data) {
				t.Fatal("fixture mutation did not change evidence")
			}
			if err = os.WriteFile(file, []byte(changed), 0600); err != nil {
				t.Fatal(err)
			}
			if id, err := LoadReady(path, scope); err == nil || id != "" {
				t.Fatal("conflicting evidence accepted", id, err)
			}
		})
	}
}

func TestLoadReadyRejectsNonprivateOrSymlinkDirectory(t *testing.T) {
	path, scope := readyJournalFixture(t)
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadReady(link, scope); err == nil {
		t.Fatal("symlink binding accepted")
	}
	if err := os.Chmod(path, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadReady(path, scope); err == nil {
		t.Fatal("public binding accepted")
	}
}
