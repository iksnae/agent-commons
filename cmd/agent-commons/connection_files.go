// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
)

type connectionConfig struct {
	Version   int    `json:"version"`
	Identity  string `json:"identity"`
	Target    string `json:"target"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	Socket    string `json:"socket"`
	TokenFile string `json:"tokenFile"`
	State     string `json:"state"`
}

func privateRead(path string, v any) error {
	return privateReadLimit(path, v, 64<<10)
}

func privateReadLimit(path string, v any, limit int64) error {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return errors.New("configuration must be a private regular file")
	}
	// Read one byte beyond the bound so truncation cannot masquerade as EOF.
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > limit {
		return fmt.Errorf("private JSON exceeds %d bytes", limit)
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err = d.Decode(v); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("one configuration object required")
	}
	return nil
}

// Never overwrite existing credential/config files, including on repeated enroll.
func createPrivate(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	return err
}
