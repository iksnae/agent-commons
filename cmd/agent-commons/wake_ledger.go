// SPDX-License-Identifier: MPL-2.0

package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const maxWakeLedger = 16 << 20

func (w *wakeWriter) save() error {
	data, err := json.Marshal(w.records)
	if err != nil {
		return err
	}
	if len(data) > maxWakeLedger {
		return errors.New("wake ledger exceeds 16 MiB; operator maintenance required")
	}
	f, err := os.CreateTemp(filepath.Dir(w.path), ".wake-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(f.Name(), w.path); err != nil {
		return err
	}
	d, err := os.Open(filepath.Dir(w.path))
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
