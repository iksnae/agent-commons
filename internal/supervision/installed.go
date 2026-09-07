// SPDX-License-Identifier: MPL-2.0

package supervision

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

// Installed records the exact service file this version installed. It contains
// paths, never credentials. Options are retained for migration, not re-rendered.
type Installed struct {
	Version int     `json:"version"`
	Options Options `json:"options"`
	Plan    Plan    `json:"plan"`
}

func InstallFile(directory string, options Options) (string, error) {
	if !safePath(directory) {
		return "", fmt.Errorf("absolute service directory required")
	}
	directory = filepath.Clean(directory)
	info, err := os.Lstat(directory)
	if err != nil {
		return "", err
	}
	if !info.IsDir() || info.Mode().Perm()&0022 != 0 {
		return "", fmt.Errorf("service directory must be real and not group/world writable")
	}
	plan, err := Render(options)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(Installed{Version: 1, Options: options, Plan: plan})
	if err != nil {
		return "", err
	}
	if len(data) > 128<<10 {
		return "", fmt.Errorf("service receipt exceeds 128 KiB")
	}
	file := filepath.Join(directory, plan.Filename)
	for _, path := range []string{file, file + ".json"} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			return "", fmt.Errorf("service path already exists or cannot be inspected: %s", path)
		}
	}
	// Claim the receipt first; existing or partial installations are never replaced.
	if err = writeExclusive(file+".json", []byte(`{"version":0}`)); err != nil {
		return "", err
	}
	if err = writeExclusive(file, []byte(plan.Content)); err != nil {
		return "", fmt.Errorf("service receipt retained at %s.json; installation incomplete: %w", file, err)
	}
	if err = completeReceipt(file+".json", data); err != nil {
		return "", fmt.Errorf("incomplete service installation retained at %s: %w", file, err)
	}
	if err = syncServiceDirectory(directory); err != nil {
		return "", err
	}
	return file, nil
}

func ReadInstalled(file string) (Installed, error) {
	var installed Installed
	if !safePath(file) {
		return installed, fmt.Errorf("absolute service file required")
	}
	data, err := readServiceFile(file + ".json")
	if err != nil {
		return installed, err
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err = d.Decode(&installed); err != nil {
		return installed, err
	}
	if d.Decode(new(any)) != io.EOF || installed.Version != 1 {
		return installed, fmt.Errorf("unsupported or invalid service receipt")
	}
	plan, err := Render(installed.Options)
	if err != nil {
		return installed, err
	}
	if plan.Label != installed.Plan.Label || plan.Filename != installed.Plan.Filename || filepath.Base(file) != plan.Filename {
		return installed, fmt.Errorf("service receipt identity mismatch")
	}
	content, err := readServiceFile(file)
	if err != nil {
		return installed, err
	}
	if string(content) != installed.Plan.Content {
		return installed, fmt.Errorf("service file changed; refusing supervisor operation")
	}
	return installed, nil
}

// RetainFiles requires the caller to stop and disable the verified native job.
// It moves configuration aside, never the service's state directory.
func RetainFiles(file string) (string, error) {
	if _, err := ReadInstalled(file); err != nil {
		return "", err
	}
	directory := filepath.Dir(file)
	retained, err := os.MkdirTemp(directory, ".agent-commons-removed-")
	if err != nil {
		return "", err
	}
	for _, path := range []string{file, file + ".json"} {
		if err = os.Rename(path, filepath.Join(retained, filepath.Base(path))); err != nil {
			return retained, fmt.Errorf("partial move retained at %s: %w", retained, err)
		}
	}
	for _, path := range []string{retained, directory} {
		if err = syncServiceDirectory(path); err != nil {
			return retained, err
		}
	}
	return retained, nil
}

func readServiceFile(path string) ([]byte, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return nil, fmt.Errorf("service files must be regular and mode 0600")
	}
	data, err := io.ReadAll(io.LimitReader(f, (128<<10)+1))
	if len(data) > 128<<10 {
		return nil, fmt.Errorf("service file exceeds 128 KiB")
	}
	return data, err
}

func writeExclusive(path string, data []byte) error {
	return writeServiceFile(path, data, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
}

func completeReceipt(path string, data []byte) error {
	return writeServiceFile(path, data, os.O_WRONLY|os.O_TRUNC|syscall.O_NOFOLLOW)
}

func writeServiceFile(path string, data []byte, flags int) error {
	f, err := os.OpenFile(path, flags, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func syncServiceDirectory(path string) error {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
