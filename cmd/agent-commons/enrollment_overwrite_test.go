// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// checkExisting refuses to overwrite a connection file whose contents differ,
// so one role can never silently claim another's recorded connection. That
// guard moved after the enroll RPC when the identity correction landed, and it
// now compares a deliberately mutated config — this pins that the refusal
// still fires, and still fires before anything on disk is touched.
func TestEnrollRefusesToOverwriteDifferingConfigWithoutWriting(t *testing.T) {
	state := onboardingService(t)
	target := t.TempDir()
	args := []string{"enroll", "--json", "--state", state, "--target", target, "--name", "lead", "--role", "lead"}

	var enrolled struct {
		Config string `json:"config"`
	}
	if err := json.Unmarshal(onboardingCommand(t, args...), &enrolled); err != nil {
		t.Fatal(err)
	}
	var written connectionConfig
	if err := privateRead(enrolled.Config, &written); err != nil {
		t.Fatal(err)
	}

	// Replace the recorded connection with one describing a different role,
	// as a second identity resolving to this path would.
	forged := written
	forged.Role = "someone-else"
	body, err := json.Marshal(forged)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(enrolled.Config, body, 0o600); err != nil {
		t.Fatal(err)
	}
	// Remove the credential so its absence afterwards proves the refusal
	// happened before anything was written. Comparing contents cannot show
	// that: ensureCredential rewrites identical bytes, so an ordering
	// regression would leave the file byte-identical either way.
	if err := os.Remove(written.TokenFile); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	err = run(context.Background(), args, nil, &out, &errOut)
	if err == nil {
		t.Fatal("enroll overwrote a differing connection file instead of refusing")
	}
	if !strings.Contains(err.Error(), "refusing overwrite") {
		t.Fatalf("refused for the wrong reason: %v", err)
	}

	after, err := os.ReadFile(enrolled.Config)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, body) {
		t.Fatal("refused enroll still rewrote the connection file")
	}
	if _, err := os.Stat(written.TokenFile); !os.IsNotExist(err) {
		t.Fatal("refused enroll wrote the credential before refusing")
	}
}
