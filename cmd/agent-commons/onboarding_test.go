// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"agentcommons/internal/core"
	"agentcommons/internal/transport"
)

func onboardingService(t *testing.T) string {
	t.Helper()
	state, err := os.MkdirTemp("", "commons-onboard-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(state) })
	service, err := core.New(state)
	if err != nil {
		t.Fatal(err)
	}
	operator, _ := service.Token("operator")
	if err = os.WriteFile(filepath.Join(state, "operator.token"), []byte(operator), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- transport.Serve(ctx, service, filepath.Join(state, "service.sock")) }()
	t.Cleanup(func() { cancel(); <-done; service.Close() })
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, err = os.Stat(filepath.Join(state, "service.sock")); err == nil {
			return state
		}
		if time.Now().After(deadline) {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond)
	}
}

func onboardingCommand(t *testing.T, args ...string) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := run(context.Background(), args, nil, &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestEnrollmentPreservesIdentityAndCredential(t *testing.T) {
	state := onboardingService(t)
	target := t.TempDir()
	args := []string{"enroll", "--state", state, "--target", target, "--name", "lead", "--role", "lead"}
	first := onboardingCommand(t, args...)
	second := onboardingCommand(t, args...)
	if !bytes.Equal(first, second) {
		t.Fatal("repeat enrollment changed connection")
	}
	var result struct {
		Config string `json:"config"`
	}
	if err := json.Unmarshal(first, &result); err != nil {
		t.Fatal(err)
	}
	var config connectionConfig
	if err := privateRead(result.Config, &config); err != nil {
		t.Fatal(err)
	}
	credential, err := readToken(config.TokenFile)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(first, []byte(credential)) {
		t.Fatal("credential leaked in enrollment output")
	}
}

func TestCheckInRecoversWelcomeWithoutAcknowledging(t *testing.T) {
	state := onboardingService(t)
	enrolled := onboardingCommand(t, "enroll", "--state", state, "--target", t.TempDir(), "--name", "lead", "--role", "lead")
	var result struct {
		Config string `json:"config"`
	}
	json.Unmarshal(enrolled, &result)
	checked := onboardingCommand(t, "check-in", "--config", result.Config, "--runtime", "claude", "--native-session", "native-one")
	var snapshot struct {
		Attachment core.Attachment `json:"attachment"`
		Inbox      struct {
			Messages []core.Delivery `json:"messages"`
		} `json:"inbox"`
	}
	if err := json.Unmarshal(checked, &snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Inbox.Messages) != 2 {
		t.Fatal("expected one welcome and one getting-started message")
	}
	for _, message := range snapshot.Inbox.Messages {
		if message.Acknowledged || message.Provenance != "service-onboarding" {
			t.Fatal("check-in changed read state or provenance")
		}
	}
	if err := run(context.Background(), []string{"check-in", "--config", result.Config, "--runtime", "codex", "--native-session", "native-two"}, nil, io.Discard, io.Discard); err == nil {
		t.Fatal("concurrent role takeover allowed")
	}
}
