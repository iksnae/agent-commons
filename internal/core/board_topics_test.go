// SPDX-License-Identifier: MPL-2.0

package core

import "testing"

func TestBoardSupportsIdeasStrategiesAndExperimentsWithoutGrantingAuthority(t *testing.T) {
	s, dir, target := setup(t)
	rpc(t, s, "operator", "sessions.register", Session{ID: "outsider", Target: t.TempDir(), Runtime: "hermes", Mode: "manual"})
	ids := map[string]string{}
	for _, topic := range []string{"learning", "technique", "pitfall", "strategy", "idea", "experiment"} {
		p := map[string]any{"topic": topic, "title": "Proposal", "text": "Peer contribution", "evidence": "Unverified notes", "idempotencyKey": topic}
		post := rpc(t, s, "lead", "board.post", p).(BoardPost)
		if post.Author != "lead" || post.GrantsAuthority || post.Topic != topic {
			t.Fatal("topic lost provenance", post)
		}
		if rpc(t, s, "lead", "board.post", p).(BoardPost).ID != post.ID {
			t.Fatal("duplicate post")
		}
		denied(t, s, "outsider", "board.get", map[string]any{"id": post.ID, "target": target})
		ids[topic] = post.ID
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for topic, id := range ids {
		page := rpc(t, s, "reviewer", "board.list", map[string]any{"topic": topic}).(map[string]any)
		posts := page["posts"].([]BoardPost)
		if len(posts) != 1 || posts[0].ID != id || posts[0].GrantsAuthority {
			t.Fatal("topic filter or persistence failed", topic)
		}
	}
}

func TestBoardRejectsUnknownTopicForPostingAndFiltering(t *testing.T) {
	s, _, _ := setup(t)
	denied(t, s, "lead", "board.post", map[string]any{"topic": "verified-fact", "title": "Title", "text": "Text", "idempotencyKey": "invalid"})
	denied(t, s, "lead", "board.list", map[string]any{"topic": "experiments"})
}
