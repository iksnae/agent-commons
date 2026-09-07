// SPDX-License-Identifier: MPL-2.0

package installation

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func Verify(target string) error {
	if !filepath.IsAbs(target) {
		return fmt.Errorf("installation path must be absolute")
	}
	target = filepath.Clean(target)
	files, err := inspectFiles(target, true)
	if err != nil {
		return err
	}
	f, err := regularFile(filepath.Join(target, receiptName))
	if err != nil {
		return err
	}
	defer f.Close()
	d := json.NewDecoder(io.LimitReader(f, maxReceipt+1))
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.Size() > maxReceipt {
		return fmt.Errorf("installation receipt exceeds 1 MiB")
	}
	d.DisallowUnknownFields()
	var r receipt
	if err = d.Decode(&r); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return fmt.Errorf("one receipt required")
	}
	if r.Version != 1 || len(r.Files) != len(files) {
		return fmt.Errorf("receipt version or file count mismatch")
	}
	var total int64
	for i, record := range r.Files {
		if record.Path != files[i] {
			return fmt.Errorf("receipt file inventory mismatch")
		}
		hash, n, err := digestFile(filepath.Join(target, record.Path))
		if err != nil {
			return err
		}
		total += n
		if total > maxBundle {
			return fmt.Errorf("installed bundle exceeds size limit")
		}
		if hash != record.SHA256 {
			return fmt.Errorf("installed file changed: %s", record.Path)
		}
		info, err := os.Lstat(filepath.Join(target, record.Path))
		if err != nil {
			return err
		}
		if uint32(info.Mode().Perm()) != record.Mode {
			return fmt.Errorf("installed file permissions changed: %s", record.Path)
		}
	}
	return nil
}

// Remove takes a verified bundle out of its original path without deleting it.
// It does not stop services; callers must stop jobs using this path first.
func Remove(target string) (string, error) {
	target = filepath.Clean(target)
	if err := Verify(target); err != nil {
		return "", err
	}
	archive, err := os.MkdirTemp(filepath.Dir(target), ".agent-commons-removed-")
	if err != nil {
		return "", err
	}
	retained := filepath.Join(archive, "installation")
	if err = os.Rename(target, retained); err != nil {
		return "", err
	}
	for _, path := range []string{archive, filepath.Dir(target)} {
		if err = syncDirectory(path); err != nil {
			return retained, fmt.Errorf("installation retained at %s but directory sync failed: %w", retained, err)
		}
	}
	return retained, nil
}
