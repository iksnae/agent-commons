// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"

	"agentcommons/internal/core"
)

func TestLaunchCheckInReusesIdentityWithoutReadingOrAcknowledgingInbox(t *testing.T) {
	state := onboardingService(t)
	target := t.TempDir()
	enrolled := onboardingCommand(t, "enroll", "--state", state, "--target", target, "--name", "lead", "--role", "lead")
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
	readInbox := func() json.RawMessage {
		data, err := rpcCall[json.RawMessage](context.Background(), connection.client, "inbox.page", struct{}{})
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	before := readInbox()
	for i := 0; i < 2; i++ {
		input, _ := json.Marshal(launchEvent{Event: "SessionStart", Session: "native-launch", Directory: target, Source: "resume"})
		var output bytes.Buffer
		if err = runLaunchContext(context.Background(), []string{"--config", enrollment.Config, "--claude-version", "2.1.236"}, bytes.NewReader(input), &output, io.Discard); err != nil {
			t.Fatal(err)
		}
		if !json.Valid(output.Bytes()) || bytes.Contains(output.Bytes(), []byte(connection.client.token)) || bytes.Contains(output.Bytes(), []byte("leaseId")) {
			t.Fatal("invalid or secret-bearing hook output")
		}
	}
	if !bytes.Equal(before, readInbox()) {
		t.Fatal("launch altered inbox")
	}
	peers, err := rpcCall[[]core.Session](context.Background(), connection.client, "sessions.list", struct{}{})
	if err != nil || len(peers) != 1 || peers[0].Attachment.NativeID != "native-launch" {
		t.Fatal("identity not reused", err)
	}
	bad, _ := json.Marshal(launchEvent{Event: "SessionStart", Session: "another", Directory: t.TempDir(), Source: "startup"})
	if err = runLaunchContext(context.Background(), []string{"--config", enrollment.Config, "--claude-version", "2.1.236"}, bytes.NewReader(bad), io.Discard, io.Discard); err == nil {
		t.Fatal("cross-project launch accepted")
	}
}
