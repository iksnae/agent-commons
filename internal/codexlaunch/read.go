// SPDX-License-Identifier: MPL-2.0

package codexlaunch

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// LoadReady reads complete local evidence for an independently verified scope.
// It does not verify native state, authorize attachment, repair partial records,
// or release the reservation. The caller must still check native root metadata.
func LoadReady(path string, expected Scope) (string, error) {
	if err := expected.validate(); err != nil {
		return "", err
	}
	root, err := openBinding(path)
	if err != nil {
		return "", err
	}
	defer root.Close()
	return loadReady(root, expected)
}

func loadReady(root *os.Root, expected Scope) (string, error) {
	var reservation struct {
		Version int   `json:"version"`
		Scope   Scope `json:"scope"`
	}
	if err := readCheckpoint(root, "reservation.json", &reservation); err != nil {
		return "", err
	}
	if reservation.Version != 1 || reservation.Scope != expected {
		return "", errors.New("binding version or scope differs from verified role")
	}
	created, err := checkpointIdentity(root, "created.json")
	if err != nil {
		return "", err
	}
	ready, err := checkpointIdentity(root, "ready.json")
	if err != nil {
		return "", err
	}
	if created != ready {
		return "", errors.New("binding ready identity differs from created identity")
	}
	return ready, nil
}

func checkpointIdentity(root *os.Root, name string) (string, error) {
	var record struct {
		ThreadID string `json:"threadId"`
	}
	if err := readCheckpoint(root, name, &record); err != nil {
		return "", err
	}
	if !threadID.MatchString(record.ThreadID) {
		return "", fmt.Errorf("%s requires an exact thread ID", name)
	}
	return record.ThreadID, nil
}

func openBinding(path string) (*os.Root, error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("binding path must be absolute")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("binding must be a real private directory")
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	opened, err := root.Stat(".")
	if err != nil || !os.SameFile(info, opened) || opened.Mode().Perm()&0077 != 0 {
		_ = root.Close()
		return nil, errors.New("binding directory changed while opening")
	}
	return root, nil
}
