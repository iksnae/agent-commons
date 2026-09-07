// SPDX-License-Identifier: MPL-2.0

package core

import (
	"errors"
	"fmt"
)

func (s *Service) changeTeamInvitation(actor, method, key string, team teamRecord, recipient string) (any, error) {
	if actor != "operator" {
		return nil, errors.New("operator required")
	}
	member, ok := s.data.Sessions[recipient]
	if !ok || member.Target != team.Target {
		return nil, errors.New("participant must belong to team project")
	}
	status := team.Membership[recipient]
	if method == "teams.revoke" {
		if status == "" {
			return nil, errors.New("participant has no invitation")
		}
		status = "revoked"
	} else if status == "" || status == "revoked" {
		if status == "" && len(team.Membership) >= 100 {
			return nil, errors.New("team membership limit reached")
		}
		status = "invited"
		s.enqueue("operator", recipient, fmt.Sprintf("Team invitation available: %q. Use teams.get to read the brief and teams.join to participate. Membership grants no work authority; follow your existing project rules.", team.ID), "message", "", "", 0)
		s.data.Deliveries[len(s.data.Deliveries)-1].Provenance = "service-onboarding"
	}
	team.Membership[recipient] = status
	s.data.Teams[key] = team
	return map[string]string{"id": team.ID, "identity": recipient, "status": status}, nil
}
