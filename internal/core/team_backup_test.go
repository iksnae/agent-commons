// SPDX-License-Identifier: MPL-2.0

package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func expectedTeamBackup(t *testing.T, s *Service) (string, []byte) {
	t.Helper()
	data, err := json.Marshal(s.data)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(data)
	return filepath.Join(s.dir, "pre-team-schema-1-"+hex.EncodeToString(hash[:])+".json"), data
}

func TestFirstTeamPreservesRestorablePrivateCoreState(t *testing.T) {
	s, _, target := setup(t)
	path, before := expectedTeamBackup(t, s)
	rpc(t, s, "operator", "teams.create", map[string]any{"id": "product", "target": target, "title": "Team", "text": "Brief"})
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, before) {
		t.Fatal("pre-team state not preserved", err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("backup not private", err)
	}
	restore := t.TempDir()
	if err = os.WriteFile(filepath.Join(restore, "state.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(restore)
	if err != nil {
		t.Fatal("backup cannot restore core state", err)
	}
	defer reopened.Close()
	if reopened.data.SchemaVersion != 1 || len(reopened.data.Teams) != 0 || reopened.data.Tokens["lead"] != s.data.Tokens["lead"] {
		t.Fatal("restored state differs from pre-team identity")
	}
	rpc(t, s, "operator", "teams.create", map[string]any{"id": "second", "target": target, "title": "Team", "text": "Brief"})
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("later team changed original backup", err)
	}
}

func TestUnsafeTeamBackupBlocksSchemaUpgrade(t *testing.T) {
	for _, kind := range []string{"conflict", "trailing", "public", "symlink", "directory", "fifo"} {
		t.Run(kind, func(t *testing.T) {
			s, _, target := setup(t)
			path, data := expectedTeamBackup(t, s)
			var err error
			switch kind {
			case "conflict":
				err = os.WriteFile(path, []byte("retained evidence"), 0600)
			case "trailing":
				err = os.WriteFile(path, append(data, ' '), 0600)
			case "public":
				err = os.WriteFile(path, data, 0644)
			case "symlink":
				err = os.Symlink("state.json", path)
			case "directory":
				err = os.Mkdir(path, 0700)
			case "fifo":
				err = syscall.Mkfifo(path, 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			denied(t, s, "operator", "teams.create", map[string]any{"id": "product", "target": target, "title": "Team", "text": "Brief"})
			if s.data.SchemaVersion != 1 || len(s.data.Teams) != 0 {
				t.Fatal("backup failure changed schema or teams")
			}
		})
	}
}

func TestMatchingPreTeamBackupCanBeReusedWithoutReplacement(t *testing.T) {
	s, _, target := setup(t)
	path, data := expectedTeamBackup(t, s)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	rpc(t, s, "operator", "teams.create", map[string]any{"id": "product", "target": target, "title": "Team", "text": "Brief"})
	after, err := os.Stat(path)
	if err != nil || !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("matching backup replaced or rewritten", err)
	}
}
