// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"testing"
	"time"
)

func TestWatcherArgumentsPreserveRuntimeSelection(t *testing.T) {
	connection := projectConnection{config: connectionConfig{State: "state", Socket: "socket", TokenFile: "credential"}}
	args := connection.watchArguments(onboardingOptions{Runtime: "codex", NativeSession: testThread, Once: true})
	expected := []string{"--state", "state", "--socket", "socket", "--token-file", "credential", "--codex-thread", testThread, "--once"}
	if !slices.Equal(args, expected) {
		t.Fatalf("unexpected watcher arguments: %v", args)
	}
	args = connection.watchArguments(onboardingOptions{Runtime: "claude"})
	if slices.Contains(args, "--codex-thread") || slices.Contains(args, "--once") {
		t.Fatal("Claude watcher gained Codex flags")
	}
}

func TestCanceledHoldReleasesAttachment(t *testing.T) {
	state := onboardingService(t)
	enrolled := onboardingCommand(t, "enroll", "--state", state, "--target", t.TempDir(), "--name", "lead", "--role", "lead")
	var result struct {
		Config string `json:"config"`
	}
	if err := json.Unmarshal(enrolled, &result); err != nil {
		t.Fatal(err)
	}
	connection, err := openProjectConnection(result.Config)
	if err != nil {
		t.Fatal(err)
	}
	options := onboardingOptions{Runtime: "claude", NativeSession: "first"}
	attachment, err := connection.attach(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = connection.holdAttachment(ctx, attachment, options, commandStreams{Output: io.Discard, Errors: io.Discard})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled watch: %v", err)
	}
	options.NativeSession = "second"
	if _, err = connection.attach(context.Background(), options); err != nil {
		t.Fatalf("lease not released: %v", err)
	}
}

func TestFailedCheckInCleanupReleasesOnlyNewLease(t *testing.T) {
	state := onboardingService(t)
	enrolled := onboardingCommand(t, "enroll", "--state", state, "--target", t.TempDir(), "--name", "lead", "--role", "lead")
	var result struct {
		Config string `json:"config"`
	}
	if err := json.Unmarshal(enrolled, &result); err != nil {
		t.Fatal(err)
	}
	connection, err := openProjectConnection(result.Config)
	if err != nil {
		t.Fatal(err)
	}
	options := onboardingOptions{Runtime: "claude", NativeSession: "failed-check-in"}
	attachment, err := connection.attach(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	connection.releaseIfNew(attachment)
	options.NativeSession = "replacement"
	if _, err = connection.attach(context.Background(), options); err != nil {
		t.Fatalf("new attachment blocked after failed check-in cleanup: %v", err)
	}
}

func TestFailedCheckInCleanupPreservesReusedLease(t *testing.T) {
	state := onboardingService(t)
	enrolled := onboardingCommand(t, "enroll", "--state", state, "--target", t.TempDir(), "--name", "lead", "--role", "lead")
	var result struct {
		Config string `json:"config"`
	}
	if err := json.Unmarshal(enrolled, &result); err != nil {
		t.Fatal(err)
	}
	connection, err := openProjectConnection(result.Config)
	if err != nil {
		t.Fatal(err)
	}
	options := onboardingOptions{Runtime: "claude", NativeSession: "same-session"}
	_, err = connection.attach(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	reused, err := connection.attach(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	if reused.Acquired {
		t.Fatal("same native session was reported as newly acquired")
	}
	connection.releaseIfNew(reused)
	options.NativeSession = "other-session"
	if _, err = connection.attach(context.Background(), options); err == nil {
		t.Fatal("reused lease was released by failed check-in cleanup")
	}
}

func TestLeaseRenewalReportsConnectionFailure(t *testing.T) {
	lease := attachmentLease{client: rpcClient{socket: "/nonexistent/commons-test.sock", token: "fixture"}}
	ticks := make(chan time.Time, 1)
	ticks <- time.Now()
	if err := lease.renewOnTicks(context.Background(), ticks); err == nil {
		t.Fatal("renewal failure hidden")
	}
}
