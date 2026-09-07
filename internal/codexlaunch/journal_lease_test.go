// SPDX-License-Identifier: MPL-2.0

package codexlaunch

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPreparationExcludesResumeUntilOwnerCloses(t *testing.T) {
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parent, "binding")
	scope := Scope{Identity: "lead", Target: "/project", Home: "/private/codex"}
	j := NewDirectoryJournal(path)
	defer j.Close()
	if err := j.Reserve(scope); err != nil {
		t.Fatal(err)
	}
	for _, checkpoint := range []func() error{
		func() error { return nil },
		func() error { return j.Created(savedThread) },
		func() error { return j.Ready(savedThread) },
	} {
		if err := checkpoint(); err != nil {
			t.Fatal(err)
		}
		lease, err := AcquireReady(path, scope)
		if lease != nil {
			_ = lease.Close()
		}
		if !errors.Is(err, ErrBindingBusy) {
			t.Fatal("preparation admitted concurrent resume", err)
		}
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}
	lease, err := AcquireReady(path, scope)
	if err != nil || lease.ThreadID != savedThread {
		t.Fatal("closed preparation did not release ready identity", err)
	}
	defer lease.Close()
	if err := j.Ready(savedThread); err == nil {
		t.Fatal("closed owner wrote another checkpoint")
	}
}

func TestClosingPartialPreparationPreservesEvidenceAndReleasesLock(t *testing.T) {
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	j := NewDirectoryJournal(filepath.Join(parent, "binding"))
	defer j.Close()
	if err := j.Reserve(Scope{Identity: "lead", Target: "/project", Home: "/private/codex"}); err != nil {
		t.Fatal(err)
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}
	root, err := openBinding(j.path)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	f, err := lockBinding(root)
	if err != nil {
		t.Fatal("partial preparation leaked lock", err)
	}
	defer f.Close()
	if _, err := os.Stat(filepath.Join(j.path, "reservation.json")); err != nil {
		t.Fatal("partial evidence removed", err)
	}
	if err := j.Created(savedThread); err == nil {
		t.Fatal("closed preparation accepted native identity")
	}
}
