// SPDX-License-Identifier: MPL-2.0

package core

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestBoardNewestPaginationPreservesScopeAndInsertionOrder(t *testing.T) {
	s := &Service{data: state{Tokens: map[string]string{"reader": "test"}, Sessions: map[string]Session{"reader": {Target: "/project"}}, Board: []BoardPost{
		{ID: "a", Target: "/project", Topic: "idea"},
		{ID: "foreign", Target: "/other", Topic: "idea"},
		{ID: "b", Target: "/project", Topic: "technique"},
		{ID: "c", Target: "/project", Topic: "idea"},
	}}}
	page := boardListing(t, s, `{"order":"newest","limit":1,"topic":"idea"}`)
	assertBoardIDs(t, page, "c")
	if page["nextCursor"] != "c" || page["order"] != "newest" {
		t.Fatal("missing paging contract", page)
	}
	// A later contribution belongs on the next fresh check-in, not an older page.
	s.data.Board = append(s.data.Board, BoardPost{ID: "d", Target: "/project", Topic: "idea"})
	page = boardListing(t, s, `{"order":"newest","limit":1,"topic":"idea","cursor":"c"}`)
	assertBoardIDs(t, page, "a")
	if page["nextCursor"] != "" {
		t.Fatal("unexpected continuation", page)
	}
	assertBoardIDs(t, boardListing(t, s, `{"order":"newest"}`), "d", "c", "b", "a")
	assertBoardIDs(t, boardListing(t, s, `{}`), "a", "b", "c", "d")
	for _, raw := range []string{`{"order":"random"}`, `{"order":"newest","cursor":"foreign"}`} {
		if _, err := s.Call("reader", "board.list", json.RawMessage(raw)); err == nil {
			t.Fatal("accepted invalid ordering or foreign cursor", raw)
		}
	}
}

func boardListing(t *testing.T, s *Service, raw string) map[string]any {
	t.Helper()
	result, err := s.Call("reader", "board.list", json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	return result.(map[string]any)
}

func assertBoardIDs(t *testing.T, page map[string]any, want ...string) {
	t.Helper()
	ids := []string{}
	for _, post := range page["posts"].([]BoardPost) {
		ids = append(ids, post.ID)
	}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("post IDs = %v, want %v", ids, want)
	}
}
