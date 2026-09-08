// SPDX-License-Identifier: MPL-2.0

package core

import "fmt"

func (s *Service) backupBeforeTeams() error {
	if s.data.SchemaVersion >= 2 {
		return nil
	}
	return s.backupSchema("pre-team-schema-1-")
}

func (s *Service) enableTeamWork(teamID string) error {
	if teamID == "" || s.data.SchemaVersion >= 3 {
		return nil
	}
	if err := s.backupSchema(fmt.Sprintf("pre-team-work-schema-%d-", s.data.SchemaVersion)); err != nil {
		return err
	}
	s.data.SchemaVersion = 3
	return nil
}
