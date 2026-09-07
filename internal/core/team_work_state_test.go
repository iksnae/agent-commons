// SPDX-License-Identifier: MPL-2.0

package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func teamWorkBackup(t *testing.T, s *Service) (string, []byte) {
	t.Helper()
	data, err := json.Marshal(s.data)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(data)
	return filepath.Join(s.dir, "pre-team-work-schema-2-"+hex.EncodeToString(hash[:])+".json"), data
}

func TestTeamWorkUpgradePreservesSchemaTwoAndDoesNotDowngrade(t *testing.T) {
	s, dir, target := setup(t)
	joinWorkTeam(t, s, target, "product", "lead", "builder", "reviewer", "red")
	backup, previous := teamWorkBackup(t, s)
	task := rpc(t, s, "lead", "tasks.assign", teamAssignment()).(Task)
	data, err := os.ReadFile(backup)
	if err != nil || !bytes.Equal(data, previous) {
		t.Fatal("pre-upgrade evidence not preserved", err)
	}
	info, err := os.Stat(backup)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("backup not private", err)
	}
	restore := t.TempDir()
	if err := os.WriteFile(filepath.Join(restore, "state.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	old, err := New(restore)
	if err != nil {
		t.Fatal("backup cannot be restored", err)
	}
	defer old.Close()
	if old.data.SchemaVersion != 2 || len(old.data.Tasks) != 0 {
		t.Fatal("backup includes new scoped work")
	}
	rpc(t, s, "operator", "teams.create", map[string]any{"id": "other", "target": target, "title": "Other", "text": "Other team"})
	if s.data.SchemaVersion != 3 {
		t.Fatal("team creation downgraded schema")
	}
	s.Close()
	reopened, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if got := rpc(t, reopened, "lead", "tasks.get", map[string]any{"id": task.ID}).(Task); got.TeamID != "product" {
		t.Fatal("restart lost task scope")
	}
}

func TestTeamWorkBackupFailurePreventsMutation(t *testing.T) {
	s, _, target := setup(t)
	joinWorkTeam(t, s, target, "product", "lead", "builder", "reviewer", "red")
	backup, before := teamWorkBackup(t, s)
	if err := os.Mkdir(backup, 0700); err != nil {
		t.Fatal(err)
	}
	denied(t, s, "lead", "tasks.assign", teamAssignment())
	after, err := json.Marshal(s.data)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("failed migration changed authoritative state", err)
	}
}

func TestCorruptTeamWorkRejectedWithoutRewritingState(t *testing.T) {
	for _, corruption := range []string{"downgrade", "task-team", "delivery-team", "context-team", "task-participant", "delivery-participant"} {
		t.Run(corruption, func(t *testing.T) {
			s, dir, target := setup(t)
			joinWorkTeam(t, s, target, "product", "lead", "builder", "reviewer", "red")
			task := rpc(t, s, "lead", "tasks.assign", teamAssignment()).(Task)
			rpc(t, s, "lead", "context.put", map[string]any{"teamId": "product", "id": "brief", "text": "Private", "expectedVersion": 0})
			switch corruption {
			case "downgrade":
				s.data.SchemaVersion = 2
			case "task-team":
				task.TeamID = ""
				s.data.Tasks[task.ID] = task
			case "task-participant":
				task.Reviewer = "missing"
				s.data.Tasks[task.ID] = task
			case "delivery-participant":
				s.data.Deliveries[len(s.data.Deliveries)-1].From = "missing"
			case "delivery-team":
				s.data.Deliveries[len(s.data.Deliveries)-1].TeamID = "missing"
			case "context-team":
				for key, versions := range s.data.Contexts {
					versions[0].TeamID = ""
					s.data.Contexts[key] = versions
				}
			}
			data, err := json.Marshal(s.data)
			if err != nil {
				t.Fatal(err)
			}
			s.Close()
			path := filepath.Join(dir, "state.json")
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			if reopened, err := New(dir); err == nil {
				reopened.Close()
				t.Fatal("corrupt team scope accepted")
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(data, after) {
				t.Fatal("corrupt state evidence rewritten", err)
			}
		})
	}
}
