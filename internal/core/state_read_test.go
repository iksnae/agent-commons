// SPDX-License-Identifier: MPL-2.0

package core

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestStartupRejectsIncompleteStateWithoutReplacingIt(t *testing.T) {
	for _, text := range []string{"null", "{}", `{"SchemaVersion":1}`, `{"SchemaVersion":-1}`, `{"SchemaVersion":1,"Sessions":{},"Tokens":{},"Tasks":{},"Contexts":{},"Keys":{}}`, ""} {
		t.Run(text, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "state.json")
			if err := os.WriteFile(path, []byte(text), 0600); err != nil {
				t.Fatal(err)
			}
			if service, err := New(dir); err == nil {
				_ = service.Close()
				t.Fatal("incomplete state accepted")
			}
			after, err := os.ReadFile(path)
			if err != nil || string(after) != text {
				t.Fatal("rejected state overwritten", err)
			}
		})
	}
}

func TestStartupRejectsOversizedStateWithoutReadingAllOfIt(t *testing.T) {
	dir := t.TempDir()
	f, err := os.OpenFile(filepath.Join(dir, "state.json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Truncate((64 << 20) + 1); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	_ = f.Close()
	if service, err := New(dir); !errors.Is(err, errStateTooLarge) {
		if service != nil {
			_ = service.Close()
		}
		t.Fatal("oversized state not rejected at size boundary", err)
	}
}

func TestStartupRejectsFIFOWithoutWaitingForWriter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := syscall.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		service, err := New(dir)
		if service != nil {
			_ = service.Close()
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("FIFO accepted")
		}
	case <-time.After(3 * time.Second):
		// Unblock the old blocking-open implementation before reporting failure.
		f, err := os.OpenFile(path, os.O_RDWR|syscall.O_NONBLOCK, 0)
		if err == nil {
			defer f.Close()
		}
		select {
		case <-done:
		case <-time.After(3 * time.Second):
		}
		t.Fatal("startup waited for a FIFO writer")
	}
}
