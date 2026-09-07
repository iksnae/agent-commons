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

func (s *Service) deliveryRunnable(d Delivery) bool {
	if !s.deliveryAccess(d.To, d) || !s.deliveryAccess(d.From, d) {
		return false
	}
	if d.TaskID != "" && d.TeamID != "" {
		return s.taskTeamActive(s.data.Tasks[d.TaskID])
	}
	return true
}
