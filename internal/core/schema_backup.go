// SPDX-License-Identifier: MPL-2.0

package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// backupSchema retains a private pre-migration snapshot before a feature
// advances the state schema. More than one feature depends on it, so the
// failure it reports names the migration that asked for it, derived from the
// caller's own file prefix rather than a hardcoded feature name.
func (s *Service) backupSchema(prefix string) error {
	data, err := json.Marshal(s.data)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(data)
	path := filepath.Join(s.dir, prefix+hex.EncodeToString(hash[:])+".json")
	if err = preserveStateBackup(path, data); err != nil {
		return fmt.Errorf("%s backup failed; schema unchanged, inspect retained evidence: %w", strings.TrimRight(prefix, "-0123456789"), err)
	}
	dir, err := os.Open(s.dir)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
