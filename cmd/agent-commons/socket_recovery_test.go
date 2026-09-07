// SPDX-License-Identifier: MPL-2.0

package main

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func socketFixture(t *testing.T) (string, string, *net.UnixListener) {
	t.Helper()
	state, err := os.MkdirTemp("", "ac-socket-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(state) })
	socket := filepath.Join(state, "service.sock")
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socket, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	listener.SetUnlinkOnClose(false)
	t.Cleanup(func() { listener.Close() })
	return state, socket, listener
}

func TestSocketRecoveryRemovesOnlyStaleDefault(t *testing.T) {
	state, socket, listener := socketFixture(t)
	if err := recoverServiceSocket(state, socket); err == nil {
		t.Fatal("active socket removed")
	}
	listener.Close()
	if err := recoverServiceSocket(state, socket); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(socket); !os.IsNotExist(err) {
		t.Fatal("stale socket remains")
	}
}

func TestSocketRecoveryPreservesCustomPath(t *testing.T) {
	state, socket, listener := socketFixture(t)
	listener.Close()
	if err := recoverServiceSocket(filepath.Join(state, "different-state"), socket); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(socket); err != nil {
		t.Fatal("custom socket removed", err)
	}
}

func TestSocketRecoveryRejectsRegularFileAndSymlink(t *testing.T) {
	state := t.TempDir()
	socket := filepath.Join(state, "service.sock")
	if err := os.WriteFile(socket, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := recoverServiceSocket(state, socket); err == nil {
		t.Fatal("regular file accepted")
	}
	data, err := os.ReadFile(socket)
	if err != nil || string(data) != "preserve" {
		t.Fatal("file changed")
	}
	if err = os.Rename(socket, filepath.Join(state, "saved")); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(filepath.Join(state, "saved"), socket); err != nil {
		t.Fatal(err)
	}
	if err = recoverServiceSocket(state, socket); err == nil {
		t.Fatal("symlink accepted")
	}
}
