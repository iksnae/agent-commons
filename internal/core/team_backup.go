// SPDX-License-Identifier: MPL-2.0

package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

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

func (s *Service) backupSchema(prefix string) error {
	data, err := json.Marshal(s.data)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(data)
	path := filepath.Join(s.dir, prefix+hex.EncodeToString(hash[:])+".json")
	if err = preserveStateBackup(path, data); err != nil {
		return fmt.Errorf("pre-team backup failed; schema unchanged, inspect retained evidence: %w", err)
	}
	dir, err := os.Open(s.dir)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
