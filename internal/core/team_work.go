// SPDX-License-Identifier: MPL-2.0

package core

func (s *Service) teamAccess(actor, target, teamID string) bool {
	if teamID == "" {
		return true
	}
	team, ok := s.data.Teams[target+"\x00"+teamID]
	return ok && (actor == "operator" || team.Membership[actor] == "joined")
}

func (s *Service) taskTeamActive(task Task) bool {
	for _, actor := range []string{task.Lead, task.Author, task.Reviewer, task.RedTeam} {
		if actor != "" && !s.teamAccess(actor, task.Target, task.TeamID) {
			return false
		}
	}
	return true
}

func (s *Service) deliveryTarget(d Delivery) string {
	if d.To == "operator" {
		return s.data.Sessions[d.From].Target
	}
	return s.data.Sessions[d.To].Target
}

func (s *Service) deliveryAccess(actor string, d Delivery) bool {
	return s.teamAccess(actor, s.deliveryTarget(d), d.TeamID)
}

// taskAbandoned reports whether a delivery carries the ID of a task the
// operator terminated. A TaskID absent from Tasks yields the zero Task, whose
// empty status is never "abandoned", so a map miss is safe. Both callers share
// this helper so the queue and the retry gate cannot drift apart.
func (s *Service) taskAbandoned(d Delivery) bool {
	return d.TaskID != "" && s.data.Tasks[d.TaskID].Status == "abandoned"
}

func (s *Service) deliveryRunnable(d Delivery) bool {
	if !s.deliveryAccess(d.To, d) || !s.deliveryAccess(d.From, d) {
		return false
	}
	// An abandoned task is terminal, so no delivery carrying its ID may run,
	// whatever its kind. Claiming task work would move the task back to
	// "working" and undo the operator's decision; claiming the result that
	// reports it would spend a managed session's paid runtime turn on
	// terminated work. Runnability governs both whether Claim takes a
	// delivery and whether Finish records the result of one already running.
	// A delivery still pending is only left alone: it stays readable in the
	// recipient's inbox with its text intact. One already in flight when the
	// operator abandons is failed by Finish with its output discarded, which
	// is what a team change also does to work in flight. The retry gate is
	// where the two part: messages.retry refuses an abandoned task outright,
	// while after a team change it still accepts the call and only leaves the
	// delivery unrunnable. Either way the text already sent stays readable.
	// Acceptance is deliberately not checked here, because result
	// deliveries carry the task ID of tasks that legitimately reach accepted.
	if s.taskAbandoned(d) {
		return false
	}
	if d.TaskID != "" && d.TeamID != "" {
		return s.taskTeamActive(s.data.Tasks[d.TaskID])
	}
	return true
}
