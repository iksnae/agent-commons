// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"agentcommons/internal/core"
)

// TestEnrollAdoptsLegacySessionIdentity is the regression guard for the
// enroll writer bug: sessions.enroll may ADOPT a pre-existing identity
// (server-side) that differs from the client's pre-RPC guessed ID, and the
// connection file must record the real, adopted session ID rather than the
// guessed one — while the file's PATH must remain pinned to the
// name/role/target-derived stem so ambient resolution (which never makes an
// RPC) can still find it.
func TestEnrollAdoptsLegacySessionIdentity(t *testing.T) {
	state := onboardingService(t)
	target := t.TempDir()
	resolvedTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	operator, err := readToken(filepath.Join(state, "operator.token"))
	if err != nil {
		t.Fatal(err)
	}
	client := rpcClient{socket: filepath.Join(state, "service.sock"), token: operator}

	const legacyID = "publisher-lead"
	if _, err := rpcCall[struct {
		Session core.Session `json:"session"`
		Token   string       `json:"token"`
	}](context.Background(), client, "sessions.register", core.Session{
		ID: legacyID, Target: resolvedTarget, Name: "", Role: "lead",
		Runtime: "manual", Mode: "manual", Policy: "coordination",
	}); err != nil {
		t.Fatal(err)
	}

	args := []string{"enroll", "--json", "--state", state, "--target", target, "--name", "lead", "--role", "lead"}
	first := onboardingCommand(t, args...)

	var result struct {
		Identity string `json:"identity"`
		Config   string `json:"config"`
	}
	if err := json.Unmarshal(first, &result); err != nil {
		t.Fatal(err)
	}
	if result.Identity != legacyID {
		t.Fatalf("enroll output identity = %q, want adopted session ID %q", result.Identity, legacyID)
	}

	var config connectionConfig
	if err := privateRead(result.Config, &config); err != nil {
		t.Fatal(err)
	}
	if config.Identity != legacyID {
		t.Fatalf("(a) connection file identity = %q, want the REAL adopted session ID %q, not a derived hash", config.Identity, legacyID)
	}

	// (b) The path must still be exactly what deriveEnrolledConnectionPath
	// computes from a manifest with no RPC involved — this must hold even
	// though the identity field now diverges from the path stem.
	manifest := projectDefaults{Version: 1, ProjectRoot: resolvedTarget, Name: "lead", Role: "lead", State: state}
	derivedPath, err := deriveEnrolledConnectionPath(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if derivedPath != result.Config {
		t.Fatalf("(b) deriveEnrolledConnectionPath = %q, want the path enroll actually wrote %q; the path must stay pinned to name/role, not follow the adopted identity", derivedPath, result.Config)
	}

	// (c) A second enroll of the same role must be idempotent and must not
	// fail with "existing config differs; refusing overwrite" now that the
	// on-disk identity is the adopted ID rather than the guessed hash.
	second := onboardingCommand(t, args...)
	var secondResult struct {
		Identity string `json:"identity"`
	}
	if err := json.Unmarshal(second, &secondResult); err != nil {
		t.Fatal(err)
	}
	if secondResult.Identity != legacyID {
		t.Fatalf("(c) re-enroll identity drifted: got %q, want %q", secondResult.Identity, legacyID)
	}
}
