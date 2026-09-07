// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"context"
	"testing"

	"agentcommons/internal/core"
)

func TestManagedPromptCarriesTeamScopeForExplicitReplies(t *testing.T) {
	fixture(t, "codex", `input=$(cat)
case "$input" in *'"TeamID":"product"'*) ;; *) exit 4;; esac
printf '%s\n' '{"type":"thread.started","thread_id":"fixture"}' '{"type":"item.completed","item":{"type":"agent_message","text":"Scope retained"}}' '{"type":"turn.completed"}'
`)
	_, output, err := (CLI{}).Run(context.Background(), core.Session{ID: "role", Mode: "managed", Runtime: "codex", Target: t.TempDir()}, core.Delivery{TeamID: "product", Text: "Inspect"})
	if err != nil || output != "Scope retained" {
		t.Fatal("managed assignment omitted team scope", err)
	}
}
