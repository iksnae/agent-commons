// SPDX-License-Identifier: MPL-2.0

package core

import (
	"errors"
	"path/filepath"
	"strings"
)

func (s *Service) validateTeams() error {
	if s.data.SchemaVersion < 2 && len(s.data.Teams) > 0 {
		return errors.New("team records require schema 2")
	}
	for key, team := range s.data.Teams {
		if !checkID(team.ID) || !filepath.IsAbs(team.Target) || key != team.Target+"\x00"+team.ID || strings.TrimSpace(team.Title) == "" || len(team.Title) > 512 || strings.TrimSpace(team.Brief) == "" || len(team.Brief) > 8192 || team.Membership == nil || len(team.Membership) > 100 {
			return errors.New("invalid saved team")
		}
		for identity, status := range team.Membership {
			member, ok := s.data.Sessions[identity]
			if !ok || member.Target != team.Target {
				return errors.New("invalid saved team participant")
			}
			switch status {
			case "invited", "joined", "left", "revoked":
			default:
				return errors.New("invalid saved membership status")
			}
		}
	}
	return nil
}
