// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"agentcommons/internal/core"
)

// When enroll ADOPTED a pre-existing session, the connection file's path stays
// pinned to the target/name/role digest while the identity is something else
// entirely. The report must not name paths derived from the identity: they do
// not exist, and after a reinstatement the operator's only remedy is a manual
// edit of exactly these files, so a wrong path aims a repair at nothing.
func TestRetireOmitsPathsItCannotLocateForAnAdoptedIdentity(t *testing.T) {
	state := onboardingService(t)
	target := t.TempDir()
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	operator, err := readToken(filepath.Join(state, "operator.token"))
	if err != nil {
		t.Fatal(err)
	}
	client := rpcClient{socket: filepath.Join(state, "service.sock"), token: operator, state: state}

	// A legacy operator-registered session with no name; enroll adopts it, so
	// the settled identity differs from the digest the path was pinned to.
	const legacyID = "publisher-lead"
	if _, err := rpcCall[struct {
		Session core.Session `json:"session"`
		Token   string       `json:"token"`
	}](context.Background(), client, "sessions.register", core.Session{
		ID: legacyID, Target: resolved, Role: "lead", Runtime: "manual", Mode: "manual", Policy: "coordination",
	}); err != nil {
		t.Fatal(err)
	}
	enrolled := enrolledRole(t, state, target, "lead", "lead")
	if enrolled.config.Identity != legacyID {
		t.Fatalf("fixture did not adopt: identity is %q, want %q", enrolled.config.Identity, legacyID)
	}
	// The real connection lives at the guessed-stem path, which is NOT derived
	// from legacyID. That divergence is the whole point of this test.
	if strings.Contains(enrolled.path, identityDigest(legacyID)) {
		t.Fatalf("fixture path %q is derived from the adopted identity; there is nothing to test", enrolled.path)
	}

	document := retirementDocument(t, "retire", "--json", "--state", state,
		"--id", legacyID, "--evidence", "Withdrawing an adopted identity.")

	if document["connection"] != "" || document["credential"] != "" {
		t.Fatalf("report named paths it cannot derive for an adopted identity: %q / %q",
			document["connection"], document["credential"])
	}
	if !strings.Contains(document["notice"], "cannot be located") {
		t.Fatalf("report omits the paths without saying why: %q", document["notice"])
	}
	// A path that WOULD have been printed by the old derivation must not appear
	// anywhere, since no such file exists.
	wrong := filepath.Join(state, "connection-"+identityDigest(legacyID)+".json")
	if strings.Contains(string(mustMarshal(t, document)), wrong) {
		t.Fatalf("report still carries the underivable path %q", wrong)
	}

	// The human form must not render an empty field either.
	var human bytes.Buffer
	if err := run(context.Background(), []string{"reinstate", "--state", state,
		"--id", legacyID, "--evidence", "And back again."}, nil, &human, io.Discard); err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"Connection", "Credential"} {
		if strings.Contains(human.String(), label) {
			t.Fatalf("human summary rendered an empty %s field:\n%s", label, human.String())
		}
	}
}

// The documented way to obtain a reinstated credential has to actually work.
// This asserts the procedure the notice and the guides tell an operator to run,
// rather than trusting prose about it.
func TestReinstateThroughCallYieldsTheCredentialTheSubcommandWithholds(t *testing.T) {
	state := onboardingService(t)
	target := t.TempDir()
	enrolled := enrolledRole(t, state, target, "exp-alpha", "builder")
	retirementDocument(t, "retire", "--json", "--state", state,
		"--id", enrolled.config.Identity, "--evidence", "Withdrawn.")

	var out bytes.Buffer
	params := `{"id":"` + enrolled.config.Identity + `","evidence":"Returned through call."}`
	if err := run(context.Background(), []string{"call", "--state", state, "sessions.reinstate", params},
		nil, &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	var response struct {
		Session core.Session `json:"session"`
		Token   string       `json:"token"`
	}
	if err := json.Unmarshal(out.Bytes(), &response); err != nil {
		t.Fatalf("call did not print a decodable response: %q: %v", out.String(), err)
	}
	if response.Token == "" {
		t.Fatalf("the documented route did not yield a credential: %q", out.String())
	}
	if response.Session.ID != enrolled.config.Identity {
		t.Fatalf("call reinstated %q, want %q", response.Session.ID, enrolled.config.Identity)
	}
	// The credential it yields is the live one.
	if _, err := rpcCall[[]core.Session](context.Background(),
		rpcClient{socket: enrolled.config.Socket, token: response.Token, state: state},
		"sessions.list", struct{}{}); err != nil {
		t.Fatalf("the credential from the documented route does not authenticate: %v", err)
	}
	// And the file on disk is still the stale one, which is exactly the limit
	// the notice and the guides record.
	stale, err := readToken(enrolled.config.TokenFile)
	if err != nil {
		t.Fatal(err)
	}
	if stale == response.Token {
		t.Fatal("reinstatement wrote the credential file after all; the recorded limit is now wrong")
	}
	if _, err := rpcCall[[]core.Session](context.Background(),
		rpcClient{socket: enrolled.config.Socket, token: stale, state: state},
		"sessions.list", struct{}{}); err == nil {
		t.Fatal("the stale credential file still authenticates; the recorded consequence is wrong")
	}
}
