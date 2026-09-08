// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func TestConnectedMCPRequiresExplicitRoleConfig(t *testing.T) {
	t.Setenv("AGENT_COMMONS_CONNECTION", "")
	if err := runConnectedMCP(context.Background(), nil, nil, io.Discard, io.Discard); err == nil {
		t.Fatal("missing role configuration accepted")
	}
}

func TestConnectedMCPUsesEnrolledRole(t *testing.T) {
	state := onboardingService(t)
	data := onboardingCommand(t, "enroll", "--json", "--state", state, "--target", t.TempDir(), "--name", "lead", "--role", "lead")
	var result struct {
		Config string `json:"config"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENT_COMMONS_CONNECTION", result.Config)
	var out bytes.Buffer
	if err := runConnectedMCP(context.Background(), nil, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`), &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"result"`) {
		t.Fatal("MCP protocol response missing")
	}
}
