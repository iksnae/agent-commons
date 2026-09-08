// SPDX-License-Identifier: MPL-2.0

package installation

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
	// Installation adds nothing to PATH, so MCP manifests travel with the
	// absolute path of the binary placed here. Their receipt entries must
	// record the rewritten bytes, not the pristine source ones.
	binary := filepath.Join(target, binaryName)
	rewritten := map[string][]byte{}
	var total int64
	for _, name := range files {
		var hash string
		var n int64
		if mcpManifest(name) {
			data, err := rewriteManifestCommand(filepath.Join(bundle, name), binary)
			if err != nil {
				return err
			}
			rewritten[name] = data
			hash, n = digestBytes(data)
		} else {
			var err error
			if hash, n, err = digestFile(filepath.Join(bundle, name)); err != nil {
				return err
			}
		}
		total += n
		if total > maxBundle {
			return fmt.Errorf("bundle exceeds 256 MiB")
		}
		mode := uint32(0600)
		if name == "agent-commons" {
			mode = 0700
		}
		if strings.HasPrefix(name, "plugins/agent-commons/") {
			info, err := os.Lstat(filepath.Join(bundle, name))
			if err != nil {
				return err
			}
			if info.Mode().Perm()&0100 != 0 {
				mode = 0700
			}
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
		if err = copyPayload(bundle, target, record, rewritten[record.Path]); err != nil {
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

// copyPayload writes one recorded payload. A non-nil content replaces the
// bundle's bytes with the ones the receipt already recorded for this path.
func copyPayload(bundle, target string, record fileRecord, content []byte) error {
	var source *os.File
	if content == nil {
		opened, err := regularFile(filepath.Join(bundle, record.Path))
		if err != nil {
			return err
		}
		defer opened.Close()
		source = opened
	}
	path := filepath.Join(target, record.Path)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, os.FileMode(record.Mode))
	if err != nil {
		return err
	}
	var n int64
	if content == nil {
		n, err = io.Copy(f, io.LimitReader(source, maxFile+1))
	} else {
		var written int
		written, err = f.Write(content)
		n = int64(written)
	}
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
