// SPDX-License-Identifier: MPL-2.0

package core

import "testing"

func TestTeamMembershipRequiresInvitationAndSurvivesRestart(t *testing.T) {
	s, dir, target := setup(t)
	rpc(t, s, "operator", "sessions.register", Session{ID: "hermes", Target: target, Runtime: "hermes", Mode: "manual"})
	team := map[string]any{"id": "product", "target": target, "title": "Product team", "text": "Review changes independently."}
	rpc(t, s, "operator", "teams.create", team)
	denied(t, s, "hermes", "teams.join", map[string]any{"id": "product"})
	denied(t, s, "hermes", "teams.get", map[string]any{"id": "product"})
	rpc(t, s, "operator", "teams.invite", map[string]any{"id": "product", "target": target, "to": "hermes"})
	joined := rpc(t, s, "hermes", "teams.join", map[string]any{"id": "product"}).(TeamView)
	if joined.Status != "joined" || joined.Brief != team["text"] || len(joined.Members) != 1 || joined.GrantsAuthority {
		t.Fatal("incomplete join brief", joined)
	}
	rpc(t, s, "hermes", "teams.join", map[string]any{"id": "product"})
	rpc(t, s, "hermes", "teams.leave", map[string]any{"id": "product"})
	if view := rpc(t, s, "hermes", "teams.join", map[string]any{"id": "product"}).(TeamView); view.Status != "joined" {
		t.Fatal("rejoin failed")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	view := rpc(t, s, "hermes", "teams.get", map[string]any{"id": "product"}).(TeamView)
	if view.Status != "joined" || s.data.Sessions["hermes"].Attachment.NativeID != "" {
		t.Fatal("membership lost or attached native session")
	}
	view.Members[0].Identity = "tampered"
	if rpc(t, s, "hermes", "teams.get", map[string]any{"id": "product"}).(TeamView).Members[0].Identity != "hermes" {
		t.Fatal("team view leaked mutable state")
	}
}

func TestTeamMembershipEnforcesOperatorAndProjectBoundaries(t *testing.T) {
	s, _, target := setup(t)
	other := t.TempDir()
	rpc(t, s, "operator", "sessions.register", Session{ID: "outsider", Target: other, Runtime: "pi", Mode: "manual"})
	team := map[string]any{"id": "product", "target": target, "title": "Team", "text": "Private brief"}
	denied(t, s, "lead", "teams.create", team)
	rpc(t, s, "operator", "teams.create", team)
	denied(t, s, "operator", "teams.invite", map[string]any{"id": "product", "target": target, "to": "outsider"})
	denied(t, s, "outsider", "teams.get", map[string]any{"id": "product", "target": target})
	denied(t, s, "lead", "teams.invite", map[string]any{"id": "product", "to": "lead"})
	rpc(t, s, "operator", "teams.invite", map[string]any{"id": "product", "target": target, "to": "lead"})
	rpc(t, s, "lead", "teams.join", map[string]any{"id": "product"})
	rpc(t, s, "operator", "teams.revoke", map[string]any{"id": "product", "target": target, "to": "lead"})
	denied(t, s, "lead", "teams.join", map[string]any{"id": "product"})
	denied(t, s, "lead", "teams.get", map[string]any{"id": "product"})
	denied(t, s, "lead", "teams.revoke", map[string]any{"id": "product", "to": "builder"})
}
