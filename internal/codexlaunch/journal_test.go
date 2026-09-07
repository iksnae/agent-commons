// SPDX-License-Identifier: MPL-2.0

package codexlaunch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestJournalRetainsReservationAndRefusesRepeatPreparation(t *testing.T) {
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parent, "binding")
	journal := NewDirectoryJournal(path)
	scope := Scope{Identity: "lead", Target: "/project", Home: "/private/codex"}
	if err := journal.Reserve(scope); err != nil {
		t.Fatal(err)
	}
	if err := journal.Ready("00000000-0000-0000-0000-000000000001"); err == nil {
		t.Fatal("ready without saved identity")
	}
	if err := journal.Created("00000000-0000-0000-0000-000000000001"); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(path, "created.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = NewDirectoryJournal(path).Reserve(scope); err == nil {
		t.Fatal("restart silently reserved another thread")
	}
	after, err := os.ReadFile(filepath.Join(path, "created.json"))
	if err != nil || string(before) != string(after) {
		t.Fatal("restart altered evidence", err)
	}
	if err = journal.Ready("00000000-0000-0000-0000-000000000002"); err == nil {
		t.Fatal("ready accepted wrong identity")
	}
	if err = journal.Ready("00000000-0000-0000-0000-000000000001"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"reservation.json", "created.json", "ready.json"} {
		info, err := os.Lstat(filepath.Join(path, name))
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatal("nonprivate checkpoint", err)
		}
	}
}

func TestJournalRejectsNonprivateAndSymlinkParent(t *testing.T) {
	parent := t.TempDir()
	if err := os.Chmod(parent, 0755); err != nil {
		t.Fatal(err)
	}
	if err := NewDirectoryJournal(filepath.Join(parent, "binding")).Reserve(Scope{Identity: "lead", Target: "/project", Home: "/private/codex"}); err == nil {
		t.Fatal("public parent accepted")
	}
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(parent, link); err != nil {
		t.Fatal(err)
	}
	if err := NewDirectoryJournal(filepath.Join(link, "binding")).Reserve(Scope{Identity: "lead", Target: "/project", Home: "/private/codex"}); err == nil {
		t.Fatal("symlink parent accepted")
	}
}
