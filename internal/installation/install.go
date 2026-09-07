// SPDX-License-Identifier: MPL-2.0

package installation

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func Install(bundle, target string) error {
	if !filepath.IsAbs(bundle) || !filepath.IsAbs(target) {
		return fmt.Errorf("bundle and destination must be absolute paths")
	}
	bundle, target = filepath.Clean(bundle), filepath.Clean(target)
	parent, err := os.Lstat(filepath.Dir(target))
	if err != nil {
		return err
	}
	if !parent.IsDir() {
		return fmt.Errorf("destination parent must be a real directory, not a symlink")
	}
	files, err := inspectFiles(bundle, false)
	if err != nil {
		return err
	}
	r := receipt{Version: 1, Files: []fileRecord{}}
	var total int64
	for _, name := range files {
		hash, n, err := digestFile(filepath.Join(bundle, name))
		if err != nil {
			return err
		}
		total += n
		if total > maxBundle {
			return fmt.Errorf("bundle exceeds 256 MiB")
		}
		mode := uint32(0600)
		if name == "agent-commons" {
			mode = 0700
		}
		r.Files = append(r.Files, fileRecord{name, hash, mode})
	}
	data, err := encodeReceipt(r)
	if err != nil {
		return err
	}
	// Exclusive creation refuses existing directories, including empty ones.
	if err = os.Mkdir(target, 0700); err != nil {
		return err
	}
	for _, record := range r.Files {
		if err = copyPayload(bundle, target, record); err != nil {
			return fmt.Errorf("incomplete installation retained at %s: %w", target, err)
		}
	}
	f, err := os.OpenFile(filepath.Join(target, receiptName), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(append(data, '\n'))
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return syncInstallation(target)
}

func copyPayload(bundle, target string, record fileRecord) error {
	source, err := regularFile(filepath.Join(bundle, record.Path))
	if err != nil {
		return err
	}
	defer source.Close()
	path := filepath.Join(target, record.Path)
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, os.FileMode(record.Mode))
	if err != nil {
		return err
	}
	n, err := io.Copy(f, io.LimitReader(source, maxFile+1))
	if err == nil && n > maxFile {
		err = fmt.Errorf("payload grew beyond limit")
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	hash, _, err := digestFile(path)
	if err != nil {
		return err
	}
	if hash != record.SHA256 {
		return fmt.Errorf("bundle changed during installation: %s", record.Path)
	}
	return nil
}
