// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"io"
	"testing"
)

func TestBundleRejectsAmbiguousActions(t *testing.T) {
	for _, args := range [][]string{
		{"bundle"}, {"bundle", "unknown", "--to", "/missing"},
		{"bundle", "remove", "--to", "/missing"},
		{"bundle", "install", "--to", "/missing"},
		{"bundle", "verify", "--to", "/missing", "--from", "/other"},
		{"bundle", "verify", "--to", "/missing", "extra"},
	} {
		if err := run(context.Background(), args, nil, io.Discard, io.Discard); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
