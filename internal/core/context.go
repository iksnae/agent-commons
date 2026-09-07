// SPDX-License-Identifier: MPL-2.0

package core

import "errors"

func contextKey(target, teamID, id string) string {
	if teamID != "" {
		return target + "\x00team\x00" + teamID + "\x00" + id
	}
	return target + "\x00" + id
}

func (s *Service) context(actor, method string, p params) (any, error) {
	target, err := s.target(actor, p.Target)
	if err != nil {
		return nil, err
	}
	if !checkID(p.ID) || p.TeamID != "" && !checkID(p.TeamID) {
		return nil, errors.New("invalid context or team ID")
	}
	if !s.teamAccess(actor, target, p.TeamID) {
		return nil, errors.New("context unavailable")
	}
	key := contextKey(target, p.TeamID, p.ID)
	versions := s.data.Contexts[key]
	if method == "context.put" {
		if p.ExpectedVersion != len(versions) {
			return nil, errors.New("context version conflict")
		}
		if err := s.enableTeamWork(p.TeamID); err != nil {
			return nil, err
		}
		c := Context{TeamID: p.TeamID, ID: p.ID, Target: target, Text: p.Text, Version: len(versions) + 1}
		s.data.Contexts[key] = append(versions, c)
		return c, nil
	}
	v := p.Version
	if v == 0 {
		v = len(versions)
	}
	if v < 1 || v > len(versions) {
		return nil, errors.New("context missing")
	}
	return versions[v-1], nil
}
