// SPDX-License-Identifier: MPL-2.0

package main

import (
	"fmt"
	"strconv"
	"strings"
)

func checkClaudeHookVersion(value string) error {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return fmt.Errorf("Claude version required for reliable fork detection")
	}
	parts := strings.Split(fields[0], ".")
	if len(parts) != 3 {
		return fmt.Errorf("unrecognized Claude version")
	}
	var version [3]int
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return fmt.Errorf("unrecognized Claude version")
		}
		version[i] = n
	}
	if version[0] < 2 || version[0] == 2 && (version[1] < 1 || version[1] == 1 && version[2] < 214) {
		return fmt.Errorf("Claude Code 2.1.214 or newer required for fork detection")
	}
	return nil
}
