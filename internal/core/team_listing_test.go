// SPDX-License-Identifier: MPL-2.0

package core

import "testing"

func TestTeamListingIsScopedBoundedAndIndependentOfInboxReceipt(t *testing.T) {
	s, dir, target := setup(t)
	for _, id := range []string{"charlie", "alpha", "hidden", "bravo"} {
		rpc(t, s, "operator", "teams.create", map[string]any{"id": id, "target": target, "title": id, "text": "Brief"})
		if id != "hidden" {
			rpc(t, s, "operator", "teams.invite", map[string]any{"id": id, "target": target, "to": "lead"})
		}
	}
	for _, delivery := range s.data.Deliveries {
		if delivery.To == "lead" {
			rpc(t, s, "lead", "inbox.acknowledge", map[string]any{"messageId": delivery.ID})
		}
	}
	page := rpc(t, s, "lead", "teams.list", map[string]any{"limit": 2}).(TeamPage)
	if len(page.Teams) != 2 || page.Teams[0].ID != "alpha" || page.Teams[1].ID != "bravo" || page.NextCursor != "bravo" {
		t.Fatal("bad sorted page", page)
	}
	last := rpc(t, s, "lead", "teams.list", map[string]any{"limit": 2, "cursor": page.NextCursor}).(TeamPage)
	if len(last.Teams) != 1 || last.Teams[0].ID != "charlie" || last.NextCursor != "" {
		t.Fatal("bad final page", last)
	}
	denied(t, s, "lead", "teams.list", map[string]any{"cursor": "hidden"})
	denied(t, s, "lead", "teams.list", map[string]any{"limit": 21})
	denied(t, s, "lead", "teams.list", map[string]any{"target": t.TempDir()})
	if empty := rpc(t, s, "builder", "teams.list", map[string]any{}).(TeamPage); len(empty.Teams) != 0 || empty.Teams == nil {
		t.Fatal("uninvited teams visible", empty)
	}
	rpc(t, s, "lead", "teams.join", map[string]any{"id": "alpha"})
	rpc(t, s, "lead", "teams.leave", map[string]any{"id": "alpha"})
	rpc(t, s, "operator", "teams.revoke", map[string]any{"id": "bravo", "target": target, "to": "lead"})
	denied(t, s, "lead", "teams.list", map[string]any{"cursor": "bravo"})
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	page = rpc(t, s, "lead", "teams.list", map[string]any{}).(TeamPage)
	if len(page.Teams) != 2 || page.Teams[0].Status != "left" || page.Teams[1].Status != "invited" {
		t.Fatal("restart lost discoverable membership", page)
	}
	page.Teams[0].Title = "changed"
	if rpc(t, s, "lead", "teams.list", map[string]any{}).(TeamPage).Teams[0].Title != "alpha" {
		t.Fatal("list leaked mutable state")
	}
	all := rpc(t, s, "operator", "teams.list", map[string]any{"target": target}).(TeamPage)
	if len(all.Teams) != 4 {
		t.Fatal("operator missing project teams")
	}
}
