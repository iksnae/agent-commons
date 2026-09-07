// SPDX-License-Identifier: MPL-2.0

package codexlaunch

import (
	"errors"
	"os"
	"syscall"
)

// Root confines resolution but may resolve in-root symlinks itself. Compare
// no-follow metadata with the opened descriptor instead of relying on flags.
func openExistingBindingFile(root *os.Root, name string, flags int) (*os.File, error) {
	before, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !privateRegular(before) {
		return nil, errors.New("binding file must be a private regular file")
	}
	f, err := root.OpenFile(name, flags|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	opened, err := f.Stat()
	if err != nil || !privateRegular(opened) || !os.SameFile(before, opened) {
		_ = f.Close()
		return nil, errors.New("binding file changed while opening")
	}
	return f, nil
}

func privateRegular(info os.FileInfo) bool {
	return info.Mode().IsRegular() && info.Mode().Perm()&0077 == 0
}
