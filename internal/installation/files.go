// SPDX-License-Identifier: MPL-2.0

// Package installation handles local extracted bundles. It never downloads,
// executes, registers services or changes agent state.
package installation

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

const maxFile = 64 << 20
const maxBundle = 256 << 20
const receiptName = ".agent-commons-install.json"
const maxEntries = 4096

var requiredFiles = []string{"agent-commons", "LICENSE", "LICENSING.md", "INSTALL.md", "README.md", "source.tar.gz", "third-party-notices/go/LICENSE"}

type fileRecord struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Mode   uint32 `json:"mode"`
}

func payloadPath(path string) bool {
	if path == "." || filepath.IsAbs(path) || filepath.Clean(path) != path || strings.HasPrefix(path, ".."+string(filepath.Separator)) {
		return false
	}
	for _, name := range requiredFiles {
		if path == name {
			return true
		}
	}
	return strings.HasPrefix(path, "third-party-notices/")
}

func regularFile(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxFile {
		f.Close()
		return nil, fmt.Errorf("payload must be a regular file at most 64 MiB: %s", path)
	}
	return f, nil
}

func inspectFiles(root string, installed bool) ([]string, error) {
	info, err := os.Lstat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("bundle root must be a real directory")
	}
	files := []string{}
	entries := 0
	limit := maxEntries
	if !installed {
		limit-- // Reserve space for the receipt added during installation.
	}
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		entries++
		if entries > limit {
			return fmt.Errorf("bundle contains too many entries")
		}
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if rel == "third-party-notices" || strings.HasPrefix(rel, "third-party-notices/") {
				return nil
			}
			if installed {
				return fmt.Errorf("unrecorded directory: %s", rel)
			}
			return filepath.SkipDir
		}
		if installed && rel == receiptName {
			return nil
		}
		if !payloadPath(rel) {
			if installed {
				return fmt.Errorf("unrecorded file: %s", rel)
			}
			return nil
		}
		f, err := regularFile(path)
		if err != nil {
			return err
		}
		f.Close()
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, name := range requiredFiles {
		found := false
		for _, file := range files {
			if file == name {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("bundle missing %s", name)
		}
	}
	sort.Strings(files)
	return files, nil
}

func digestFile(path string) (string, int64, error) {
	f, err := regularFile(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, maxFile+1))
	if err != nil {
		return "", n, err
	}
	if n > maxFile {
		return "", n, fmt.Errorf("payload grew beyond limit")
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}
