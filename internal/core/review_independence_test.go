// SPDX-License-Identifier: MPL-2.0

package core

import "testing"

// Review-time self-review is already covered (service_test.go, team_work_test.go).
// These cover the assign-time half of the same guarantee: a task cannot be
// created whose reviewer or red team is the author, and red team cannot double
// as reviewer. Without this, work could be assigned with review pre-defeated.
func TestAssignRequiresIndependentReviewParticipants(t *testing.T) {
	s, _, _ := setup(t)
	assignment := func(key, reviewer, redTeam string) map[string]any {
		p := map[string]any{"to": "builder", "title": "work", "text": "inspect", "criteria": "evidence", "idempotencyKey": key, "reviewer": reviewer}
		if redTeam != "" {
			p["redTeam"] = redTeam
		}
		return p
	}
	denied(t, s, "lead", "tasks.assign", assignment("author-reviews-self", "builder", ""))
	denied(t, s, "lead", "tasks.assign", assignment("author-red-teams-self", "reviewer", "builder"))
	denied(t, s, "lead", "tasks.assign", assignment("reviewer-is-red-team", "reviewer", "reviewer"))
	denied(t, s, "lead", "tasks.assign", assignment("unknown-reviewer", "ghost", ""))
	rpc(t, s, "lead", "tasks.assign", assignment("independent", "reviewer", "red"))
}
