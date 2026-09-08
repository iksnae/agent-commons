// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"testing"
	"time"

	"agentcommons/internal/console"
	"agentcommons/internal/core"
)

// goldenTarget is the Scope of the live service the golden was captured from.
const goldenTarget = "/private/tmp/claude-501/-Users-k-Projects-agent-commons/eb8f4681-0d33-4f92-902d-351d757b6d69/scratchpad/khnum_step1_target"

// goldenSessions and goldenTasks reproduce, field for field, the service state
// that produced testdata/console_snapshot_golden.json. The two agents at
// runtime "manual" are unattached; the builder holds a current attachment
// lease. The lease expiry is expressed relative to the test's clock because
// only its ordering against now is observable in the output.
//
// The sessions are deliberately NOT listed in the order the golden prints
// them. sessions.list happens to return them sorted, so a fixture in wire
// order would let the golden pass with the row sort deleted.
func goldenSessions(now time.Time) []core.Session {
	return []core.Session{
		{Name: "rev", ID: "khnum-rev", Role: "reviewer", Runtime: "manual", Mode: "manual", Policy: "workflow", Target: goldenTarget},
		{Name: "lead", ID: "khnum-lead", Role: "lead", Runtime: "manual", Mode: "manual", Policy: "coordination", Target: goldenTarget},
		{
			Name:       "builder",
			ID:         "khnum-build",
			Role:       "builder",
			Runtime:    "manual",
			Mode:       "manual",
			Policy:     "workflow",
			Target:     goldenTarget,
			Attachment: core.Attachment{Runtime: "claude", NativeID: "khnum-native-1", LeaseID: "b4dcf555af53c3f798c7c6e3dfdce0a8f5ee3093e50b0290", ExpiresAt: now.Add(time.Hour).Unix(), Epoch: 1},
		},
	}
}

func goldenTasks() []core.Task {
	return []core.Task{{
		ID:       "84e9ffd08490f09f567b9af522df83c41df8f6f63e63c195",
		Target:   goldenTarget,
		Lead:     "operator",
		Author:   "khnum-build",
		Title:    "Sever the console output contract",
		Criteria: "cmd owns the JSON",
		Reviewer: "khnum-rev",
		Status:   "assigned",
		Revision: 0,
		Reviews:  []core.Review{},
	}}
}

// TestConsoleOnceOutputMatchesCapturedGoldenBytes pins the `console --once`
// document to bytes captured by running the v0.0.4 binary (bd4e8d1) against a
// live service in exactly this state. The expected value was not written from
// reading the code; it is that binary's stdout, byte for byte, including key
// order and the encoder's trailing newline. Moving the output contract out of
// the terminal UI model must not move a single byte.
func TestConsoleOnceOutputMatchesCapturedGoldenBytes(t *testing.T) {
	golden, err := os.ReadFile("testdata/console_snapshot_golden.json")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	var output bytes.Buffer
	load := func(context.Context) (consoleSnapshotJSON, error) {
		return newConsoleSnapshotJSON(goldenTarget, goldenSessions(now), goldenTasks(), now), nil
	}
	if err := writeConsoleSnapshot(context.Background(), load, &output); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output.Bytes(), golden) {
		t.Fatalf("console --once bytes drifted from the captured contract\n got: %q\nwant: %q", output.String(), string(golden))
	}
}

// TestConsoleSnapshotJSONRendersEveryAttachmentState covers the one row state
// the live capture could not reach: Busy is cleared when the service reloads
// its state, so no restartable fixture produces it. The expected literal was
// read out of the base tree at bd4e8d1
// (cmd/agent-commons/console.go:79, "managed delivery running") rather than
// from the code under test.
func TestConsoleSnapshotJSONRendersEveryAttachmentState(t *testing.T) {
	now := time.Now()
	peers := []core.Session{
		{Name: "busy", ID: "id-busy", Role: "builder", Runtime: "claude", Busy: true, Attachment: core.Attachment{ExpiresAt: now.Add(time.Hour).Unix()}},
		{Name: "leased", ID: "id-leased", Role: "builder", Runtime: "codex", Attachment: core.Attachment{ExpiresAt: now.Add(time.Hour).Unix()}},
		{Name: "expired", ID: "id-expired", Role: "builder", Runtime: "codex", Attachment: core.Attachment{ExpiresAt: now.Add(-time.Hour).Unix()}},
	}
	document := newConsoleSnapshotJSON("scope", peers, nil, now)
	want := []string{
		"busy / builder | claude | id-busy | managed delivery running",
		"expired / builder | codex | id-expired | not attached",
		"leased / builder | codex | id-leased | attachment lease current (reachability unverified)",
	}
	if !reflect.DeepEqual(document.Agents, want) {
		t.Fatalf("agent rows drifted\n got: %q\nwant: %q", document.Agents, want)
	}
	if document.Tasks != nil {
		t.Fatalf("absent tasks must stay absent, not become an empty list: %q", document.Tasks)
	}
}

// TestConsoleSnapshotJSONOrdersAndFormatsTaskRows covers what the golden
// cannot: it was captured from a service holding a single task, so it pins the
// task row format but says nothing about ordering between rows.
func TestConsoleSnapshotJSONOrdersAndFormatsTaskRows(t *testing.T) {
	tasks := []core.Task{
		{ID: "t-2", Status: "submitted", Revision: 3, Title: "Second"},
		{ID: "t-1", Status: "assigned", Revision: 0, Title: "First"},
	}
	document := newConsoleSnapshotJSON("scope", nil, tasks, time.Now())
	want := []string{
		"t-1 | assigned | revision 0 | First",
		"t-2 | submitted | revision 3 | Second",
	}
	if !reflect.DeepEqual(document.Tasks, want) {
		t.Fatalf("task rows drifted\n got: %q\nwant: %q", document.Tasks, want)
	}
	if document.Agents != nil {
		t.Fatalf("absent agents must stay absent, not become an empty list: %q", document.Agents)
	}
}

// TestConsoleOutputContractIsNotTheTerminalModel is the decoupling assertion.
//
// A key-set check alone would not catch the regression it guards: today
// console.Snapshot and consoleSnapshotJSON marshal to the same three keys, so
// re-pointing the writer at the UI model would keep such a test green while
// re-creating the exact defect this step removed. What is actually asserted is
// the type the writer is fed — the UI model must not be able to reach stdout —
// which is what a future field added to console.Snapshot would otherwise widen.
func TestConsoleOutputContractIsNotTheTerminalModel(t *testing.T) {
	loader := reflect.TypeOf(writeConsoleSnapshot).In(1)
	if loader.Kind() != reflect.Func || loader.NumOut() != 2 {
		t.Fatalf("writeConsoleSnapshot no longer takes a snapshot loader: %s", loader)
	}
	document := loader.Out(0)
	if document == reflect.TypeOf(console.Snapshot{}) {
		t.Fatal("writeConsoleSnapshot marshals the terminal UI model; the CLI output contract must stay cmd-owned")
	}
	if document != reflect.TypeOf(consoleSnapshotJSON{}) {
		t.Fatalf("writeConsoleSnapshot marshals %s, not the declared CLI contract type", document)
	}
	// Frozen, additive-only key set. A new key may appear below only with a
	// deliberate edit here; nothing existing may be renamed or removed.
	if keys := consoleDocumentKeys(t, consoleSnapshotJSON{}); !reflect.DeepEqual(keys, []string{"Agents", "Scope", "Tasks"}) {
		t.Fatalf("CLI output contract keys changed: %q", keys)
	}
}

func consoleDocumentKeys(t *testing.T, value any) []string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
