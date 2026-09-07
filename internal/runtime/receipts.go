// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"agentcommons/internal/core"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

func (c CLI) receipt(s core.Session, d core.Delivery, definitions []Definition, project, runtimeID string, runErr error) error {
	type source struct{ Name, Kind, Runtime, Path, BaseDir, Digest string }
	sources := make([]source, 0, len(definitions))
	selected := ""
	for _, def := range definitions {
		sources = append(sources, source{def.Name, def.Kind, def.Runtime, def.Path, def.BaseDir, def.Digest})
		if def.Kind == "agents" && def.Name == s.Role && (selected == "" || def.Runtime == s.Runtime) {
			selected = def.Path
		}
	}
	sum := sha256.Sum256([]byte(project))
	outcome := "completed"
	if runErr != nil {
		outcome = "failed"
	}
	record := struct {
		SessionID, DeliveryID, RuntimeSessionID, Target, Role, SelectedRolePath, ProjectPromptSHA256, Outcome, At string
		Definitions                                                                                               []source
	}{s.ID, d.ID, runtimeID, s.Target, s.Role, selected, hex.EncodeToString(sum[:]), outcome, time.Now().UTC().Format(time.RFC3339Nano), sources}
	dir := filepath.Join(c.StateDir, "receipts")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, "run-*.json")
	if err != nil {
		return err
	}
	err = json.NewEncoder(file).Encode(record)
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}
