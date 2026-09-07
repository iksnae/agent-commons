// SPDX-License-Identifier: MPL-2.0

package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestTeamCreationAdvancesSchemaWithoutRewritingLegacyLabels(t *testing.T) {
	s, _, target := setup(t)
	if s.data.SchemaVersion != 1 {
		t.Fatal("startup upgraded state without team creation")
	}
	rpc(t, s, "operator", "teams.create", map[string]any{"id": "team", "target": target, "title": "Team", "text": "Brief"})
	if s.data.SchemaVersion != 2 || s.data.Sessions["lead"].Team != "" {
		t.Fatal("team creation changed legacy team label or failed schema gate")
	}
	p := map[string]any{"id": "team", "target": target, "to": "lead"}
	before := len(s.data.Deliveries)
	rpc(t, s, "operator", "teams.invite", p)
	rpc(t, s, "operator", "teams.invite", p)
	if len(s.data.Deliveries) != before+1 || s.data.Deliveries[before].Acknowledged || s.data.Deliveries[before].GrantsAuthority {
		t.Fatal("duplicate or authoritative invitation")
	}
	rpc(t, s, "lead", "teams.join", map[string]any{"id": "team"})
	rpc(t, s, "lead", "teams.leave", map[string]any{"id": "team"})
	if len(s.data.Deliveries) != before+1 || s.data.Deliveries[before].Acknowledged {
		t.Fatal("membership altered inbox receipt state")
	}
}

func TestInvalidSavedTeamStateFailsWithoutRewritingEvidence(t *testing.T) {
	for _, corruption := range []string{"downgrade", "missing", "members", "status", "identity", "target", "key"} {
		t.Run(corruption, func(t *testing.T) {
			s, dir, target := setup(t)
			rpc(t, s, "operator", "teams.create", map[string]any{"id": "team", "target": target, "title": "Team", "text": "Brief"})
			canonical, err := canonicalTarget(target)
			if err != nil {
				t.Fatal(err)
			}
			key := canonical + "\x00team"
			team := s.data.Teams[key]
			if team.Membership == nil {
				t.Fatal("fixture team missing")
			}
			switch corruption {
			case "downgrade":
				s.data.SchemaVersion = 1
			case "members":
				team.Membership = nil
			case "status":
				team.Membership["lead"] = "admin"
			case "identity":
				team.Membership["unknown"] = "joined"
			case "target":
				team.Target = "/different"
			case "key":
				delete(s.data.Teams, key)
				key = "wrong"
			}
			s.data.Teams[key] = team
			if corruption == "missing" {
				s.data.Teams = nil
			}
			data, err := json.Marshal(s.data)
			if err != nil {
				t.Fatal(err)
			}
			if err = s.Close(); err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(dir, "state.json")
			if err = os.WriteFile(file, data, 0600); err != nil {
				t.Fatal(err)
			}
			if reopened, err := New(dir); err == nil {
				_ = reopened.Close()
				t.Fatal("corrupt membership state accepted")
			}
			after, err := os.ReadFile(file)
			if err != nil || string(after) != string(data) {
				t.Fatal("rejected state rewritten", err)
			}
		})
	}
}
