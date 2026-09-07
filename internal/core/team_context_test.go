// SPDX-License-Identifier: MPL-2.0

package core

import (
	"strings"
	"testing"
)

func TestTeamContextNamespacesAndPinnedDeliveryScope(t *testing.T) {
	s, dir, target := setup(t)
	joinWorkTeam(t, s, target, "product", "lead", "builder")
	rpc(t, s, "lead", "context.put", map[string]any{"id": "brief", "text": "Project context", "expectedVersion": 0})
	rpc(t, s, "lead", "context.put", map[string]any{"teamId": "product", "id": "brief", "text": "Team secret", "expectedVersion": 0})
	denied(t, s, "reviewer", "context.get", map[string]any{"teamId": "product", "id": "brief"})
	if c := rpc(t, s, "reviewer", "context.get", map[string]any{"id": "brief"}).(Context); c.Text != "Project context" || c.TeamID != "" {
		t.Fatal("context namespace collision")
	}
	args := map[string]any{"teamId": "product", "to": "builder", "text": "Read", "idempotencyKey": "context", "contextId": "brief", "contextVersion": 1}
	d := rpc(t, s, "lead", "messages.send", args).(Delivery)
	if d.TeamID != "product" || !strings.Contains(d.Text, "Team secret") || d.ContextVersion != 1 {
		t.Fatal("team context pin lost scope")
	}
	denied(t, s, "builder", "messages.send", map[string]any{"to": "lead", "text": "Reply", "replyTo": d.ID, "idempotencyKey": "reply"})
	rpc(t, s, "lead", "context.put", map[string]any{"teamId": "product", "id": "brief", "text": "New revision", "expectedVersion": 1})
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	claimed := claim(t, reopened, "builder")
	if claimed.ID != d.ID || !strings.Contains(claimed.Text, "Team secret") || strings.Contains(claimed.Text, "New revision") {
		t.Fatal("restart changed immutable pin")
	}
	if err := reopened.Finish(d.ID, "root", "Team result", nil); err != nil {
		t.Fatal(err)
	}
	result := reopened.data.Deliveries[len(reopened.data.Deliveries)-1]
	if result.TeamID != "product" || result.ReplyTo != d.ID {
		t.Fatal("automatic return lost team scope")
	}
	rpc(t, reopened, "operator", "teams.revoke", map[string]any{"id": "product", "target": target, "to": "lead"})
	denied(t, reopened, "lead", "context.get", map[string]any{"teamId": "product", "id": "brief"})
	denied(t, reopened, "lead", "messages.send", args)
	denied(t, reopened, "lead", "inbox.acknowledge", map[string]any{"messageId": result.ID})
}
