// SPDX-License-Identifier: MPL-2.0

package core

import "errors"

func (s *Service) validateTeamWork() error {
	valid := func(target, teamID string) bool {
		if teamID == "" {
			return true
		}
		_, ok := s.data.Teams[target+"\x00"+teamID]
		return s.data.SchemaVersion >= 3 && checkID(teamID) && ok
	}
	for key, task := range s.data.Tasks {
		if !valid(task.Target, task.TeamID) || task.TeamID != "" && key != task.ID {
			return errors.New("invalid saved task team scope")
		}
		if task.TeamID != "" {
			for _, actor := range []string{task.Lead, task.Author, task.Reviewer} {
				if !s.savedTeamParticipant(actor, task.Target, task.TeamID) {
					return errors.New("invalid saved team task participant")
				}
			}
			if task.RedTeam != "" && !s.savedTeamParticipant(task.RedTeam, task.Target, task.TeamID) {
				return errors.New("invalid saved team task red reviewer")
			}
		}
	}
	for key, versions := range s.data.Contexts {
		for _, c := range versions {
			if !valid(c.Target, c.TeamID) || key != contextKey(c.Target, c.TeamID, c.ID) {
				return errors.New("invalid saved context scope")
			}
		}
	}
	for _, d := range s.data.Deliveries {
		target := s.deliveryTarget(d)
		if !valid(target, d.TeamID) {
			return errors.New("invalid saved delivery team scope")
		}
		if d.TaskID != "" && s.data.Tasks[d.TaskID].TeamID != d.TeamID {
			return errors.New("saved delivery and task team scopes differ")
		}
		if d.TeamID != "" && (!s.savedTeamParticipant(d.From, target, d.TeamID) || !s.savedTeamParticipant(d.To, target, d.TeamID) || d.TaskID != "" && s.data.Tasks[d.TaskID].Target != target) {
			return errors.New("invalid saved team delivery participants or target")
		}
	}
	return nil
}

func (s *Service) savedTeamParticipant(actor, target, teamID string) bool {
	if actor == "operator" {
		return true
	}
	member, ok := s.data.Sessions[actor]
	return ok && member.Target == target && s.data.Teams[target+"\x00"+teamID].Membership[actor] != ""
}
