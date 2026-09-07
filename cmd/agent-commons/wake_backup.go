// SPDX-License-Identifier: MPL-2.0

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type wakeWriteResult struct {
	Applied   bool   `json:"applied"`
	Uncertain bool   `json:"uncertain"`
	Backup    string `json:"backup,omitempty"`
}

func (w *wakeWriter) replaceWithBackup(kept map[string]wakeRecord, report *wakeWriteResult) error {
	var err error
	report.Backup, err = w.backup()
	if err != nil {
		return err
	}
	original := w.records
	w.records = kept
	err = w.save()
	if err != nil {
		w.records = original
	}
	report.Applied = err == nil
	report.Uncertain = err != nil
	return err
}

// Keep a durable snapshot before removing any suppression history. Backups are
// never removed automatically and include operator evidence when present.
func (w *wakeWriter) backup() (string, error) {
	data, err := json.Marshal(w.records)
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp(filepath.Dir(w.path), ".wake-backup-*.json")
	if err != nil {
		return "", err
	}
	path := f.Name()
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return path, err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return path, err
	}
	defer dir.Close()
	return path, dir.Sync()
}
