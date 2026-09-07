// SPDX-License-Identifier: MPL-2.0

package codexlaunch

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

const savedThread = "00000000-0000-0000-0000-000000000001"

func TestLoadReadyReadsMatchingCompleteBindingAfterRestart(t *testing.T) {
	path, scope := readyJournalFixture(t)
	id, err := LoadReady(path, scope)
	if err != nil || id != savedThread {
		t.Fatalf("load saved identity: %q, %v", id, err)
	}
	if err = NewDirectoryJournal(path).Reserve(scope); err == nil {
		t.Fatal("reading released the exclusive reservation")
	}
}

func TestLoadReadyRejectsScopeMismatch(t *testing.T) {
	for _, field := range []string{"identity", "target", "home"} {
		t.Run(field, func(t *testing.T) {
			path, scope := readyJournalFixture(t)
			switch field {
			case "identity":
				scope.Identity = "other-role"
			case "target":
				scope.Target = "/other-project"
			case "home":
				scope.Home = "/other-home"
			}
			if id, err := LoadReady(path, scope); err == nil || id != "" {
				t.Fatal("different scope accepted", id, err)
			}
		})
	}
}

func TestLoadReadyRejectsDamagedCheckpointsWithoutRepair(t *testing.T) {
	for _, name := range []string{"reservation.json", "created.json", "ready.json"} {
		for _, damage := range []string{"missing", "null", "trailing", "oversize", "public", "symlink", "fifo"} {
			t.Run(name+"/"+damage, func(t *testing.T) {
				path, scope := readyJournalFixture(t)
				file := filepath.Join(path, name)
				damageCheckpoint(t, file, damage)
				before, beforeErr := os.Lstat(file)
				if id, err := LoadReady(path, scope); err == nil || id != "" {
					t.Fatal("damaged binding accepted", id, err)
				}
				after, afterErr := os.Lstat(file)
				if os.IsNotExist(beforeErr) && os.IsNotExist(afterErr) {
					return
				}
				if beforeErr != nil || afterErr != nil || !os.SameFile(before, after) || before.Size() != after.Size() || before.Mode() != after.Mode() || before.ModTime() != after.ModTime() {
					t.Fatal("load changed damaged evidence", beforeErr, afterErr)
				}
			})
		}
	}
}

func readyJournalFixture(t *testing.T) (string, Scope) {
	t.Helper()
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parent, "binding")
	scope := Scope{Identity: "lead", Target: "/project", Home: "/private/codex"}
	j := NewDirectoryJournal(path)
	defer j.Close()
	for _, err := range []error{j.Reserve(scope), j.Created(savedThread), j.Ready(savedThread)} {
		if err != nil {
			t.Fatal(err)
		}
	}
	return path, scope
}

func damageCheckpoint(t *testing.T, path, damage string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	switch damage {
	case "public":
		err = os.Chmod(path, 0644)
	case "missing", "symlink", "fifo":
		if err = os.Remove(path); err == nil {
			switch damage {
			case "symlink":
				if err = os.WriteFile(path+".saved", data, 0600); err == nil {
					err = os.Symlink(filepath.Base(path)+".saved", path)
				}
			case "fifo":
				err = syscall.Mkfifo(path, 0600)
			}
		}
	default:
		switch damage {
		case "null":
			data = []byte("null")
		case "trailing":
			data = append(data, []byte(" {}")...)
		case "oversize":
			data = append(data, []byte(strings.Repeat(" ", 64<<10))...)
		}
		err = os.WriteFile(path, data, 0600)
	}
	if err != nil {
		t.Fatal(err)
	}
}
