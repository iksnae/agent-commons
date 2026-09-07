// SPDX-License-Identifier: MPL-2.0

package main

import (
	"strings"
	"testing"
)

func TestLaunchEventRejectsInheritedAndMalformedInputs(t *testing.T) {
	for _, input := range []string{
		`{"hook_event_name":"SubagentStart","session_id":"native","cwd":"/project","source":"startup"}`,
		`{"hook_event_name":"SessionStart","session_id":"native","cwd":"/project","source":"fork"}`,
		`{"hook_event_name":"SessionStart","session_id":"native","agent_id":"child","cwd":"/project","source":"startup"}`,
		`{"hook_event_name":"SessionStart","session_id":"","cwd":"/project","source":"startup"}`,
		`{} {}`, strings.Repeat(" ", 65537),
	} {
		if _, err := readLaunchEvent(strings.NewReader(input)); err == nil {
			t.Fatal("accepted invalid launch", input[:min(100, len(input))])
		}
	}
}

func TestLaunchEventAcceptsNativeMetadataWithoutParsingTranscript(t *testing.T) {
	event, err := readLaunchEvent(strings.NewReader(`{"hook_event_name":"SessionStart","session_id":"native","cwd":"/project","source":"resume","transcript_path":"/never-read","future_field":true}`))
	if err != nil || event.Session != "native" {
		t.Fatal(event, err)
	}
}
