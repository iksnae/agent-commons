// SPDX-License-Identifier: MPL-2.0

package core

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"syscall"
)

const maxStateBytes = 64 << 20

var errStateTooLarge = errors.New("core state exceeds 64 MiB; no records were removed; archive and recovery tooling is required")

func readStateFile(path string) (state, error) {
	var loaded state
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return loaded, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return loaded, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return loaded, errors.New("state must be a private regular file")
	}
	if info.Size() > maxStateBytes {
		return loaded, errStateTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(f, maxStateBytes+1))
	if err != nil {
		return loaded, err
	}
	if len(data) > maxStateBytes {
		return loaded, errStateTooLarge
	}
	// Decode into zero state: missing fields in existing files are corruption,
	// not permission to generate a fresh registry and credentials.
	err = json.Unmarshal(data, &loaded)
	return loaded, err
}

func encodeState(data state) ([]byte, error) {
	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	if len(encoded) > maxStateBytes {
		return nil, errStateTooLarge
	}
	return encoded, nil
}
