// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"syscall"
	"testing"

	"agentcommons/internal/core"
)

func TestDoctorIsReadOnlyAndOmitsPeerData(t *testing.T) {
	state := onboardingService(t)
	data := onboardingCommand(t, "enroll", "--state", state, "--target", t.TempDir(), "--name", "lead", "--role", "lead")
	var enrollment struct {
		Config string `json:"config"`
	}
	if err := json.Unmarshal(data, &enrollment); err != nil {
		t.Fatal(err)
	}
	connection, err := openProjectConnection(enrollment.Config)
	if err != nil {
		t.Fatal(err)
	}
	before, err := rpcCall[json.RawMessage](context.Background(), connection.client, "sessions.list", struct{}{})
	if err != nil {
		t.Fatal(err)
	}
	result := onboardingCommand(t, "doctor", "--config", enrollment.Config)
	var report healthReport
	if err = json.Unmarshal(result, &report); err != nil {
		t.Fatal(err)
	}
	if !report.Ready || len(report.Checks) != 6 || report.Runtime == nil || len(report.Runtime.Sessions) != 1 || report.Runtime.Sessions[0].Identity != report.Identity {
		t.Fatalf("bad health: %s", result)
	}
	for _, secret := range []string{connection.client.token, "Welcome to Agent Commons", "Getting started:"} {
		if bytes.Contains(result, []byte(secret)) {
			t.Fatal("doctor exposed credential or peer text")
		}
	}
	after, err := rpcCall[json.RawMessage](context.Background(), connection.client, "sessions.list", struct{}{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("doctor mutated registry/attachment")
	}
	inbox, err := rpcCall[struct {
		Messages []core.Delivery `json:"messages"`
	}](context.Background(), connection.client, "inbox.page", map[string]bool{"unreadOnly": true})
	if err != nil {
		t.Fatal(err)
	}
	if len(inbox.Messages) != 2 {
		t.Fatal("doctor acknowledged welcome messages")
	}
}

func TestDoctorFailsClosed(t *testing.T) {
	var out bytes.Buffer
	err := run(context.Background(), []string{"doctor", "--config", filepath.Join(t.TempDir(), "missing")}, nil, &out, io.Discard)
	if err == nil || !bytes.Contains(out.Bytes(), []byte(`"ready":false`)) {
		t.Fatalf("false health: %v %s", err, out.String())
	}
	for _, args := range [][]string{{"doctor"}, {"doctor", "--config", "/missing", "--timeout", "0s"}, {"doctor", "--config", "/missing", "--timeout", "2m"}} {
		if err = run(context.Background(), args, nil, io.Discard, io.Discard); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}

func TestDoctorReportsSafeFailureCode(t *testing.T) {
	var out bytes.Buffer
	err := run(context.Background(), []string{"doctor", "--config", filepath.Join(t.TempDir(), "missing")}, nil, &out, io.Discard)
	if err == nil || !bytes.Contains(out.Bytes(), []byte(`"code":"not_found"`)) {
		t.Fatalf("missing actionable code: %v %s", err, out.String())
	}
	if bytes.Contains(out.Bytes(), []byte("missing")) {
		t.Fatal("diagnostic exposed path details")
	}
}

func TestDoctorRejectsFIFOConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fifo")
	if err := syscall.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	if err := run(context.Background(), []string{"doctor", "--config", path, "--timeout", "1ms"}, nil, io.Discard, io.Discard); err == nil {
		t.Fatal("FIFO accepted")
	}
}
