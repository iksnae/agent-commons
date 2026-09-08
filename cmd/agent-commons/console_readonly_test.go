// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestConsoleDoesNotAttachAcknowledgeOrExposeCredentials(t *testing.T) {
	state := onboardingService(t)
	enrolled := onboardingCommand(t, "enroll", "--json", "--state", state, "--target", t.TempDir(), "--name", "lead", "--role", "lead")
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
	if !bytes.Equal(registry, read("sessions.list")) || !bytes.Equal(inbox, read("inbox.page")) {
		t.Fatal("console changed attachment or inbox")
	}
}
