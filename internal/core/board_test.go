package core

import "testing"

func TestBoardScopePersistenceAndIdempotency(t *testing.T) {
	s, dir, target := setup(t)
	rpc(t, s, "operator", "sessions.register", Session{ID: "manual", Target: target, Mode: "manual", Runtime: "claude"})
	rpc(t, s, "operator", "sessions.register", Session{ID: "other", Target: t.TempDir(), Mode: "manual", Runtime: "claude"})
	p := map[string]any{"topic": "technique", "title": "Wake", "text": "Use a fixed notification", "evidence": "live probe", "idempotencyKey": "one"}
	a := rpc(t, s, "manual", "board.post", p).(BoardPost)
	if a.Author != "manual" || a.GrantsAuthority {
		t.Fatal(a)
	}
	if rpc(t, s, "manual", "board.post", p).(BoardPost).ID != a.ID {
		t.Fatal("duplicate post")
	}
	p["text"] = "changed"
	denied(t, s, "manual", "board.post", p)
	denied(t, s, "other", "board.get", map[string]any{"id": a.ID})
	p["replyTo"] = a.ID
	p["idempotencyKey"] = "two"
	rpc(t, s, "manual", "board.post", p)
	page := rpc(t, s, "lead", "board.list", map[string]any{"query": "wake", "limit": 1}).(map[string]any)
	if page["nextCursor"] != a.ID {
		t.Fatal(page)
	}
	denied(t, s, "other", "board.list", map[string]any{"cursor": a.ID})
	s.Close()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if rpc(t, s, "lead", "board.get", map[string]any{"id": a.ID}).(BoardPost).Text != "Use a fixed notification" {
		t.Fatal("post lost")
	}
}
