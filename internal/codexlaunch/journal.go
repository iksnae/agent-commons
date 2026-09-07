// SPDX-License-Identifier: MPL-2.0

package codexlaunch

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

type DirectoryJournal struct {
	path     string
	reserved bool
	created  string
	lease    *BindingLease
	closed   bool
}

// NewDirectoryJournal opens no files. Reserve exclusively creates the directory;
// existing or interrupted preparations are never overwritten or replayed. Call
// Close only after the owned native process stops, including on preparation error.
func NewDirectoryJournal(path string) *DirectoryJournal { return &DirectoryJournal{path: path} }

func (j *DirectoryJournal) Reserve(scope Scope) error {
	if j.closed || j.reserved {
		return errors.New("preparation journal is closed or already reserved")
	}
	if err := scope.validate(); err != nil {
		return err
	}
	parent := filepath.Dir(j.path)
	info, err := os.Lstat(parent)
	if err != nil {
		return err
	}
	if !filepath.IsAbs(j.path) || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return errors.New("binding requires an absolute path in a real private directory")
	}
	if err = os.Mkdir(j.path, 0700); err != nil {
		return err
	}
	root, err := openBinding(j.path)
	if err != nil {
		return err
	}
	f, err := lockBinding(root)
	_ = root.Close()
	if err != nil {
		return err
	}
	j.lease = &BindingLease{file: f}
	if err = syncDirectory(parent); err != nil {
		return err
	}
	if err = j.checkpoint("reservation.json", struct {
		Version int   `json:"version"`
		Scope   Scope `json:"scope"`
	}{1, scope}); err != nil {
		return err
	}
	j.reserved = true
	return nil
}

func (j *DirectoryJournal) Created(id string) error {
	if j.closed || !j.reserved || j.created != "" || !threadID.MatchString(id) {
		return errors.New("created checkpoint requires reserved preparation and exact thread ID")
	}
	if err := j.checkpoint("created.json", map[string]string{"threadId": id}); err != nil {
		return err
	}
	j.created = id
	return nil
}

func (j *DirectoryJournal) Ready(id string) error {
	if j.closed || id == "" || j.created != id {
		return errors.New("ready checkpoint differs from saved thread identity")
	}
	return j.checkpoint("ready.json", map[string]string{"threadId": id})
}

func (j *DirectoryJournal) checkpoint(name string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(j.path, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return syncDirectory(j.path)
}

func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
