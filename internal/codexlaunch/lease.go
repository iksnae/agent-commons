// SPDX-License-Identifier: MPL-2.0

package codexlaunch

import (
	"errors"
	"os"
	"sync"
	"syscall"
)

var ErrBindingBusy = errors.New("saved Codex binding is already in use")

// BindingLease excludes cooperating users of one binding directory. It is not a
// native thread lock or an Agent Commons role attachment. Keep it until the
// owned native process has stopped. Never unlink its persistent lock file.
type BindingLease struct {
	ThreadID string
	file     *os.File
	once     sync.Once
	err      error
}

func AcquireReady(path string, expected Scope) (*BindingLease, error) {
	if err := expected.validate(); err != nil {
		return nil, err
	}
	root, err := openBinding(path)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	f, err := lockBinding(root)
	if err != nil {
		return nil, err
	}
	id, err := loadReady(root, expected)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	return &BindingLease{ThreadID: id, file: f}, nil
}

func lockBinding(root *os.Root) (*os.File, error) {
	f, err := root.OpenFile("resume.lock", os.O_CREATE|os.O_EXCL|os.O_RDWR|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
	if errors.Is(err, os.ErrExist) {
		f, err = openExistingBindingFile(root, "resume.lock", os.O_RDWR)
	}
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		_ = f.Close()
		return nil, errors.New("binding lock must be a private regular file")
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrBindingBusy
		}
		return nil, err
	}
	return f, nil
}

func (l *BindingLease) Close() error {
	l.once.Do(func() { l.err = l.file.Close() })
	return l.err
}
