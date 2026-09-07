// SPDX-License-Identifier: MPL-2.0

package core

import (
	"bytes"
	"errors"
	"io"
	"os"
	"syscall"
)

// Preserve and sync an exclusive backup, or verify identical retained evidence.
// Partial or conflicting files remain available for operator inspection.
func preserveStateBackup(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
	if errors.Is(err, os.ErrExist) {
		return syncMatchingBackup(path, data)
	}
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	return errors.Join(err, f.Close())
}

func syncMatchingBackup(path string, expected []byte) error {
	f, err := os.OpenFile(path, os.O_RDWR|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return errors.New("backup must be a private regular file")
	}
	data, err := io.ReadAll(io.LimitReader(f, int64(len(expected))+1))
	if err != nil {
		return err
	}
	if !bytes.Equal(data, expected) {
		return errors.New("existing backup differs from pre-team state; not overwritten")
	}
	return f.Sync()
}
