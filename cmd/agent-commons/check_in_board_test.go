// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"agentcommons/internal/core"
)

func TestCheckInSurfacesNewestContributionsForEveryHarness(t *testing.T) {
	for _, runtime := range []string{"hermes", "pi", "claude", "codex"} {
		t.Run(runtime, func(t *testing.T) {
			args, connection := codexPrepareFixture(t)
			for i := range 7 {
				_, err := rpcCall[core.BoardPost](context.Background(), connection.client, "board.post", map[string]string{
					"topic": "idea", "title": fmt.Sprintf("Idea %d", i), "text": "Peer contribution", "idempotencyKey": fmt.Sprint(i),
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			output := onboardingCommand(t, "check-in", "--config", args[1], "--runtime", runtime, "--native-session", "board-test-session")
			var snapshot struct {
				Board struct {
					Posts      []core.BoardPost
					NextCursor string
					Order      string
				}
			}
			if err := json.Unmarshal(output, &snapshot); err != nil {
				t.Fatal(err)
			}
			board := snapshot.Board
			if len(board.Posts) != 5 || board.Posts[0].Title != "Idea 6" || board.Posts[4].Title != "Idea 2" || board.Order != "newest" || board.NextCursor == "" {
				t.Fatalf("check-in did not surface latest contributions: %+v", board)
			}
		})
	}
}
