// SPDX-License-Identifier: MPL-2.0

package main

import (
	"io"
	"testing"
)

func TestWakeResolutionRequiresEveryDecisionSafeguard(t *testing.T) {
	base := []string{"--config", "private.json", "--codex-thread", testThread, "--message-id", "one"}
	flags := [][]string{
		{"--expected-hash", "inspected-hash"},
		{"--decision", "retry"},
		{"--evidence", "Reviewed history"},
		{"--operator-token-file", "operator.token"},
		{"--acknowledge-notification-risk"},
	}
	for omitted := range flags {
		t.Run(flags[omitted][0], func(t *testing.T) {
			args := append([]string{}, base...)
			for i, flag := range flags {
				if i != omitted {
					args = append(args, flag...)
				}
			}
			if _, err := parseWakeResolution(args, io.Discard); err == nil {
				t.Fatal("incomplete decision accepted")
			}
		})
	}
	if _, err := parseWakeResolution(base, io.Discard); err != nil {
		t.Fatal("plain inspection rejected", err)
	}
}
