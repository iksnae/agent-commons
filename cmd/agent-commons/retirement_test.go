// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"agentcommons/internal/core"
)

// enrolledRole stands up a role through the same command that created the
// operator's real registry, so retirement is exercised against a connection
// enroll actually wrote rather than one this test invented.
type retiredRoleConnection struct {
	config connectionConfig
	// path is what enroll actually wrote, taken from its own report rather
	// than re-derived here: a test that recomputes the production derivation
	// asserts nothing about it.
	path string
}

func enrolledRole(t *testing.T, state, target, name, role string) retiredRoleConnection {
	t.Helper()
	document := onboardingCommand(t, "enroll", "--json", "--state", state,
		"--target", target, "--name", name, "--role", role)
	var enrolled struct {
		Identity string `json:"identity"`
		Config   string `json:"config"`
	}
	if err := json.Unmarshal(document, &enrolled); err != nil {
		t.Fatal(err)
	}
	var config connectionConfig
	if err := privateRead(enrolled.Config, &config); err != nil {
		t.Fatal(err)
	}
	return retiredRoleConnection{config: config, path: enrolled.Config}
}

func retirementDocument(t *testing.T, args ...string) map[string]string {
	t.Helper()
	var document map[string]string
	if err := json.Unmarshal(onboardingCommand(t, args...), &document); err != nil {
		t.Fatal(err)
	}
	return document
}

// A credential is 24 random bytes rendered as hex. An identity is "agent-"
// followed by 32 hex characters, so a bare 48-character run is a token and
// nothing else this report carries.
var credentialShape = regexp.MustCompile(`\b[0-9a-f]{48}\b`)

// The CLI created the identities that clutter the registry, so the CLI has to
// be able to withdraw one. It reports the private files rather than reaching
// for them: deleting a credential file is not this command's decision, and the
// service has already made the credential inside it useless.
func TestRetireCommandReportsInertFilesWithoutDeletingThem(t *testing.T) {
	state := onboardingService(t)
	target := t.TempDir()
	enrolled := enrolledRole(t, state, target, "exp-alpha", "builder")
	credential, err := readToken(enrolled.config.TokenFile)
	if err != nil {
		t.Fatal(err)
	}

	document := retirementDocument(t, "retire", "--json", "--state", state,
		"--id", enrolled.config.Identity, "--evidence", "One afternoon's communications test.")

	if document["identity"] != enrolled.config.Identity {
		t.Fatalf("report names identity %q, want %q", document["identity"], enrolled.config.Identity)
	}
	if document["name"] != "exp-alpha" || document["role"] != "builder" || document["target"] != enrolled.config.Target {
		t.Fatalf("report lost the attribution retirement exists to keep: %+v", document)
	}
	if document["retiredAt"] == "" || document["reason"] == "" {
		t.Fatalf("report omits the recorded retirement: %+v", document)
	}
	if document["connection"] != enrolled.path || document["credential"] != enrolled.config.TokenFile {
		t.Fatalf("report names %q/%q, want %q/%q", document["connection"], document["credential"], enrolled.path, enrolled.config.TokenFile)
	}
	for _, path := range []string{enrolled.path, enrolled.config.TokenFile} {
		if _, err := os.Lstat(path); err != nil {
			t.Fatalf("retire removed or disturbed %s: %v", path, err)
		}
	}
	if credentialShape.MatchString(string(mustMarshal(t, document))) {
		t.Fatalf("retire report carries something shaped like a credential: %+v", document)
	}

	// The file is intact and the credential in it is inert; that is the whole
	// claim the report makes, so both halves have to be true.
	if _, err := rpcCall[[]core.Session](context.Background(),
		rpcClient{socket: enrolled.config.Socket, token: credential, state: state}, "sessions.list", struct{}{}); err == nil {
		t.Fatal("the credential in the retained file still authenticates")
	}

	var human strings.Builder
	if err := run(context.Background(), []string{"retire", "--state", state, "--id", enrolled.config.Identity, "--evidence", "e"}, nil, &human, io.Discard); err == nil {
		t.Fatal("a second retirement of the same identity succeeded")
	}
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestRetireCommandRequiresIdentityAndEvidence(t *testing.T) {
	state := onboardingService(t)
	target := t.TempDir()
	enrolled := enrolledRole(t, state, target, "exp-alpha", "builder")
	for _, args := range [][]string{
		{"retire", "--state", state, "--evidence", "no identity"},
		{"retire", "--state", state, "--id", enrolled.config.Identity},
		{"retire", "--state", state, "--id", enrolled.config.Identity, "--evidence", "  "},
		{"retire", "--state", state, "--id", enrolled.config.Identity, "--evidence", "e", "extra"},
	} {
		if err := run(context.Background(), args, nil, io.Discard, io.Discard); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	// The identity must still be usable, or the refusals above proved nothing.
	if _, err := os.Lstat(enrolled.config.TokenFile); err != nil {
		t.Fatal(err)
	}
	retirementDocument(t, "retire", "--json", "--state", state, "--id", enrolled.config.Identity, "--evidence", "Now for real.")
}

// Reinstatement is a separate operator act. Its report must not carry the new
// credential: the service holds it, and a token in terminal scrollback is a
// credential in terminal scrollback.
func TestReinstateCommandRestoresTheIdentityWithoutPrintingItsCredential(t *testing.T) {
	state := onboardingService(t)
	target := t.TempDir()
	enrolled := enrolledRole(t, state, target, "exp-alpha", "builder")
	operator, err := readToken(filepath.Join(state, "operator.token"))
	if err != nil {
		t.Fatal(err)
	}
	client := rpcClient{socket: enrolled.config.Socket, token: operator, state: state}
	listed := func(params any) map[string]bool {
		t.Helper()
		peers, err := rpcCall[[]core.Session](context.Background(), client, "sessions.list", params)
		if err != nil {
			t.Fatal(err)
		}
		found := map[string]bool{}
		for _, peer := range peers {
			found[peer.ID] = true
		}
		return found
	}

	retirementDocument(t, "retire", "--json", "--state", state, "--id", enrolled.config.Identity, "--evidence", "Withdrawn.")
	if listed(struct{}{})[enrolled.config.Identity] {
		t.Fatal("retired identity is still in the default listing")
	}
	if !listed(map[string]any{"includeRetired": true})[enrolled.config.Identity] {
		t.Fatal("retired identity is unreachable even with includeRetired")
	}

	document := retirementDocument(t, "reinstate", "--json", "--state", state,
		"--id", enrolled.config.Identity, "--evidence", "The experiment resumed.")
	if document["identity"] != enrolled.config.Identity {
		t.Fatalf("report names identity %q, want %q", document["identity"], enrolled.config.Identity)
	}
	if document["retiredAt"] != "" || document["reason"] != "" {
		t.Fatalf("reinstatement report still carries a tombstone: %+v", document)
	}
	if credentialShape.MatchString(string(mustMarshal(t, document))) {
		t.Fatalf("reinstate report carries something shaped like a credential: %+v", document)
	}
	// Withholding the credential is only honest if the report says what that
	// costs. "The file was not changed" is true and useless on its own; the
	// operator needs to know the identity cannot connect until they fix it, and
	// which command does hand them the new credential.
	for _, required := range []string{"cannot connect", "by hand", "sessions.reinstate"} {
		if !strings.Contains(document["notice"], required) {
			t.Fatalf("reinstate notice omits %q, so it states the fact without the consequence: %q", required, document["notice"])
		}
	}
	if !listed(struct{}{})[enrolled.config.Identity] {
		t.Fatal("reinstated identity is missing from the default listing")
	}
	if err := run(context.Background(), []string{"reinstate", "--state", state, "--id", enrolled.config.Identity, "--evidence", "again"}, nil, io.Discard, io.Discard); err == nil {
		t.Fatal("reinstating a live identity succeeded")
	}
}

// A re-run of enroll must not quietly bring a withdrawn role back to life. The
// server resolves the identity itself, so the client's derived ID never
// protects against this; the refusal has to say what happened and what to do.
func TestEnrollRefusesToResurrectARetiredRole(t *testing.T) {
	state := onboardingService(t)
	target := t.TempDir()
	enrolled := enrolledRole(t, state, target, "exp-alpha", "builder")
	retirementDocument(t, "retire", "--json", "--state", state, "--id", enrolled.config.Identity, "--evidence", "Withdrawn.")

	var errOut strings.Builder
	err := run(context.Background(), []string{"enroll", "--json", "--state", state,
		"--target", target, "--name", "exp-alpha", "--role", "builder"}, nil, io.Discard, &errOut)
	if err == nil {
		t.Fatal("enroll resurrected a retired role")
	}
	for _, want := range []string{enrolled.config.Identity, "retired", "sessions.reinstate"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("refusal omits %q: %q", want, err)
		}
	}
}
