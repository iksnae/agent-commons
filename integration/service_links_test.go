// SPDX-License-Identifier: MPL-2.0

package integration

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func assertPersistentServiceLinks(t *testing.T, m nativeManager, present bool) {
	t.Helper()
	if runtime.GOOS != "linux" {
		return
	}
	directory, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{m.label + ".service", filepath.Join("default.target.wants", m.label+".service")} {
		path := filepath.Join(directory, "systemd", "user", relative)
		if !present {
			if _, err := os.Lstat(path); !os.IsNotExist(err) {
				t.Fatal("persistent service link remains", path, err)
			}
			continue
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil || resolved != m.file {
			t.Fatal("persistent service link does not reference installed file", path, resolved, err)
		}
	}
}
