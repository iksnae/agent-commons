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
	data, err := json.Marshal(s.data)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(data)
	path := filepath.Join(s.dir, "pre-team-schema-1-"+hex.EncodeToString(hash[:])+".json")
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
