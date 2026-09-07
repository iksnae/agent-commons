// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"io"
	"testing"
)

func TestServiceRejectsIncompatibleFlagsBeforeNativeCommands(t *testing.T) {
	for _, args := range [][]string{nil, {"remove", "--file", "/missing"}, {"start", "--binary", "/bin/true"}, {"install", "--file", "/missing"}, {"status", "--confirm-stopped"}} {
		if err := runService(context.Background(), args, io.Discard, io.Discard); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
