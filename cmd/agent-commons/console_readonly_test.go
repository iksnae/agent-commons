// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
)

func TestConsoleDoesNotAttachAcknowledgeOrExposeCredentials(t *testing.T) {
	state := onboardingService(t)
	target := t.TempDir()
	enrolled := onboardingCommand(t, "enroll", "--json", "--state", state, "--target", target, "--name", "lead", "--role", "lead")
	var enrollment struct {
		Config string `json:"config"`
	}
	if err := json.Unmarshal(enrolled, &enrollment); err != nil {
		t.Fatal(err)
	}
	connection, err := openProjectConnection(enrollment.Config)
	if err != nil {
		t.Fatal(err)
	}
	read := func(method string) json.RawMessage {
		value, err := rpcCall[json.RawMessage](context.Background(), connection.client, method, struct{}{})
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	registry, inbox := read("sessions.list"), read("inbox.page")
	result := onboardingCommand(t, "console", "--config", enrollment.Config, "--once")
	if !json.Valid(result) || bytes.Contains(result, []byte(connection.client.token)) {
		t.Fatal("invalid/credential-bearing snapshot")
	}
	// Absence of the bearer token is too narrow a check on its own: it stays
	// true while the document silently widens to carry, say, a lease ID or a
	// session's instructions. Assert the whole emitted key set instead, so any
	// new key has to be added here deliberately.
	if keys := consoleDocumentKeys(t, json.RawMessage(result)); !reflect.DeepEqual(keys, []string{"Agents", "Scope", "Tasks"}) {
		t.Fatalf("console emitted keys beyond its contract: %q", keys)
	}
	// Scope's key is asserted above and its format by the golden, but neither
	// pins where the value comes from: both stay green while production emits
	// an empty Scope for every project. Compare against the enrolled --target
	// itself, resolved the way enrollment resolves it, so the assertion does
	// not read back the same connection field the console reads.
	var document struct {
		Scope string
	}
	if err := json.Unmarshal(result, &document); err != nil {
		t.Fatal(err)
	}
	enrolledTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	emittedScope, err := filepath.EvalSymlinks(document.Scope)
	if err != nil {
		t.Fatalf("console scope %q is not the enrolled project: %v", document.Scope, err)
	}
	if emittedScope != enrolledTarget {
		t.Fatalf("console scope %q is not the enrolled project %q", document.Scope, target)
	}
	if !bytes.Equal(registry, read("sessions.list")) || !bytes.Equal(inbox, read("inbox.page")) {
		t.Fatal("console changed attachment or inbox")
	}
}
