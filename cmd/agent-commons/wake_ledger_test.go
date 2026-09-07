// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
)

func TestWakeRejectsTrailingAndOversizedLedgerWithoutReplacingIt(t *testing.T) {
	for _, data := range []string{"{}{}", "{}" + strings.Repeat(" ", (16<<20)-2) + "{}"} {
		t.Run("invalid ledger", func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Chmod(dir, 0700); err != nil {
				t.Fatal(err)
			}
			w, err := openWake(context.Background(), dir, testThread, "role", io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			path := w.path
			if err = w.lock.Close(); err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(path, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			if w, err = openWake(context.Background(), dir, testThread, "role", io.Discard); err == nil {
				_ = w.lock.Close()
				t.Fatal("invalid ledger accepted")
			}
			actual, err := os.ReadFile(path)
			if err != nil || string(actual) != data {
				t.Fatal("invalid ledger changed", err)
			}
			// A rejected read must release the binding lock for a repaired ledger.
			if err = os.WriteFile(path, []byte("{}"), 0600); err != nil {
				t.Fatal(err)
			}
			w, err = openWake(context.Background(), dir, testThread, "role", io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			_ = w.lock.Close()
		})
	}
}
