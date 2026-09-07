// SPDX-License-Identifier: MPL-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrivateReadEnforcesWholeFileSizeLimit(t *testing.T) {
	const limit = 64 << 10
	valid := `{"identity":"lead"}`
	padded := valid + strings.Repeat(" ", limit-len(valid))
	for _, tc := range []struct {
		name     string
		input    string
		accepted bool
	}{
		{"ordinary", valid, true},
		{"exact limit", padded, true},
		{"one byte over limit", padded + " ", false},
		{"hidden second object", padded + valid, false},
		{"second object within limit", valid + valid, false},
		{"unknown field", `{"unknown":"value"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "connection.json")
			if err := os.WriteFile(path, []byte(tc.input), 0600); err != nil {
				t.Fatal(err)
			}
			var config struct {
				Identity string `json:"identity"`
			}
			err := privateRead(path, &config)
			if (err == nil) != tc.accepted {
				t.Fatalf("accepted=%v, want %v; error=%v", err == nil, tc.accepted, err)
			}
			if tc.accepted && config.Identity != "lead" {
				t.Fatal("configuration identity changed")
			}
		})
	}
}
