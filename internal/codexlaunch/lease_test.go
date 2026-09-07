// SPDX-License-Identifier: MPL-2.0

package codexlaunch

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestBindingLeaseExcludesConcurrentOwnersAndReleasesOnClose(t *testing.T) {
	path, scope := readyJournalFixture(t)
	lease, err := AcquireReady(path, scope)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	if lease.ThreadID != savedThread {
		t.Fatal("wrong leased identity")
	}
	if second, err := AcquireReady(path, scope); !errors.Is(err, ErrBindingBusy) {
		if second != nil {
			_ = second.Close()
		}
		t.Fatal("concurrent owner admitted", err)
	}
	if err = lease.Close(); err != nil {
		t.Fatal(err)
	}
	if err = lease.Close(); err != nil {
		t.Fatal("repeat close failed", err)
	}
	next, err := AcquireReady(path, scope)
	if err != nil {
		t.Fatal("closed lease retained ownership", err)
	}
	defer next.Close()
	if _, err = os.Stat(filepath.Join(path, "resume.lock")); err != nil {
		t.Fatal("lock file removed", err)
	}
}

func TestBindingLeaseRejectsUnsafeLockFiles(t *testing.T) {
	for _, kind := range []string{"public", "symlink", "directory"} {
		t.Run(kind, func(t *testing.T) {
			path, scope := readyJournalFixture(t)
			lock := filepath.Join(path, "resume.lock")
			if err := os.Remove(lock); err != nil {
				t.Fatal(err)
			}
			var err error
			switch kind {
			case "public":
				err = os.WriteFile(lock, nil, 0644)
			case "symlink":
				err = os.Symlink("ready.json", lock)
			case "directory":
				err = os.Mkdir(lock, 0700)
			}
			if err != nil {
				t.Fatal(err)
			}
			if lease, err := AcquireReady(path, scope); err == nil {
				_ = lease.Close()
				t.Fatal("unsafe lock admitted")
			}
		})
	}
}

func TestBindingLeaseReleasesAfterInvalidEvidence(t *testing.T) {
	path, scope := readyJournalFixture(t)
	wrong := scope
	wrong.Identity = "another-role"
	if lease, err := AcquireReady(path, wrong); err == nil {
		_ = lease.Close()
		t.Fatal("wrong scope admitted")
	}
	lease, err := AcquireReady(path, scope)
	if err != nil {
		t.Fatal("failed acquisition leaked lock", err)
	}
	_ = lease.Close()
}
