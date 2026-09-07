// SPDX-License-Identifier: MPL-2.0

package main

import "testing"

func TestClaudeHookVersionFloor(t *testing.T) {
	for _, value := range []string{"", "1.9.999", "2.0.999", "2.1.213", "2.1.214-beta", "garbage"} {
		if checkClaudeHookVersion(value) == nil {
			t.Fatal("accepted unreliable version", value)
		}
	}
	for _, value := range []string{"2.1.214", "2.1.236 (Claude Code)", "2.2.0", "3.0.0"} {
		if err := checkClaudeHookVersion(value); err != nil {
			t.Fatal(value, err)
		}
	}
}
