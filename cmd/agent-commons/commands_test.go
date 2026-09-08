// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"os"
	"strconv"
	"strings"
	"testing"
)

// The bare-argument usage line once listed seven commands while help listed
// fifteen, so a caller reading it concluded check-in and doctor did not exist.
// Both renderings now derive from commandSections; these tests fail if that
// single source is bypassed or if it documents a command run does not dispatch.
func TestUsageListsEveryDocumentedCommand(t *testing.T) {
	usage := usageError().Error()
	for _, name := range commandNames() {
		if !strings.Contains(usage, name) {
			t.Fatalf("usage omits documented command %q: %s", name, usage)
		}
	}
	listed := strings.Split(strings.TrimPrefix(strings.Fields(usage)[2], "agent-commons"), "|")
	if len(listed) != len(commandNames()) {
		t.Fatalf("usage lists %d commands, help documents %d", len(listed), len(commandNames()))
	}
}

func TestHelpAndUsageDocumentTheSameCommands(t *testing.T) {
	var out bytes.Buffer
	if err := writeHelp(&out); err != nil {
		t.Fatal(err)
	}
	usage := usageError().Error()
	for _, name := range commandNames() {
		if !bytes.Contains(out.Bytes(), []byte(name)) {
			t.Fatalf("help omits %q that usage advertises", name)
		}
	}
	if strings.Count(usage, "|") != len(commandNames())-1 {
		t.Fatalf("usage separators disagree with the catalog: %s", usage)
	}
}

func TestEveryDocumentedCommandIsDispatched(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range commandNames() {
		if !bytes.Contains(source, []byte(strconv.Quote(name))) {
			t.Fatalf("help documents %q but run does not dispatch it", name)
		}
	}
}
