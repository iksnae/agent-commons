// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentcommons/internal/core"
)

// initReport mirrors the JSON runInit writes. It is machine-readable and
// something may already parse it, so the pre-existing keys are asserted
// alongside the new ones.
type initReport struct {
	Operation      string `json:"operation"`
	Target         string `json:"target"`
	Manifest       string `json:"manifest"`
	Config         string `json:"config"`
	State          string `json:"state"`
	Runtime        string `json:"runtime"`
	Identity       string `json:"identity"`
	IdentityStatus string `json:"identityStatus"`
	Warning        string `json:"warning"`
}

// initCommand runs init and returns both its JSON report and everything it
// wrote to stderr, because the overwrite notice must survive on both.
func initCommand(t *testing.T, args ...string) (initReport, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	if err := run(context.Background(), append([]string{"init"}, args...), nil, &out, &errOut); err != nil {
		t.Fatalf("init %v: %v", args, err)
	}
	var report initReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("init output is not machine-readable JSON: %v (%s)", err, out.String())
	}
	if report.Operation != "init" {
		t.Fatalf("init output operation = %q, want \"init\"", report.Operation)
	}
	return report, errOut.String()
}

func readManifest(t *testing.T, path string) projectDefaults {
	t.Helper()
	var manifest projectDefaults
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func sessionIDs(t *testing.T, state string) []string {
	t.Helper()
	operator, err := readToken(filepath.Join(state, "operator.token"))
	if err != nil {
		t.Fatal(err)
	}
	client := rpcClient{socket: filepath.Join(state, "service.sock"), token: operator}
	peers, err := rpcCall[[]core.Session](context.Background(), client, "sessions.list", struct{}{})
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(peers))
	for _, peer := range peers {
		ids = append(ids, peer.ID)
	}
	return ids
}

// TestInitPointsProjectAtAdoptedIdentity is the regression guard for init's
// duplicate pre-RPC guard. enrollAgent already checks the connection file
// AFTER the server tells it which identity it really adopted; init kept its
// own copy of that check running BEFORE the RPC, comparing the on-disk
// adopted ID against a derived hash that never matches it. Every identity the
// operator has is adopted, so init could not re-point a project at any of
// them.
func TestInitPointsProjectAtAdoptedIdentity(t *testing.T) {
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
		ID: legacyID, Target: resolvedTarget, Name: "", Role: "workspace-lead", Team: "publisher",
		Runtime: "manual", Mode: "manual", Policy: "coordination",
	}); err != nil {
		t.Fatal(err)
	}

	identity := []string{"--name", "publisher-lead", "--role", "workspace-lead", "--team", "publisher"}
	onboardingCommand(t, append([]string{"enroll", "--state", state, "--target", target}, identity...)...)

	report, _ := initCommand(t, append([]string{"--state", state, "--target", target, "--runtime", "claude"}, identity...)...)
	if report.Identity != legacyID {
		t.Fatalf("init identity = %q, want the adopted session ID %q", report.Identity, legacyID)
	}
	if report.IdentityStatus != "adopted" {
		t.Fatalf("init identityStatus = %q, want \"adopted\"", report.IdentityStatus)
	}
	manifest := readManifest(t, report.Manifest)
	if manifest.Name != "publisher-lead" || manifest.Runtime != "claude" || manifest.Team != "publisher" {
		t.Fatalf("manifest = %+v, want the requested name/team/runtime", manifest)
	}
}

// TestInitDistinguishesCreatedFromAdopted pins that the report says which of
// the two happened. Before this, minting a brand new identity and re-pointing
// at an existing one produced identical output.
func TestInitDistinguishesCreatedFromAdopted(t *testing.T) {
	state := onboardingService(t)
	target := t.TempDir()
	args := []string{"--state", state, "--target", target, "--name", "lead", "--role", "workspace-lead"}

	created, _ := initCommand(t, args...)
	if created.IdentityStatus != "created" {
		t.Fatalf("first init identityStatus = %q, want \"created\"", created.IdentityStatus)
	}
	if !strings.HasPrefix(created.Identity, "agent-") {
		t.Fatalf("first init identity = %q, want a freshly derived agent- identity", created.Identity)
	}

	adopted, _ := initCommand(t, args...)
	if adopted.IdentityStatus != "adopted" {
		t.Fatalf("repeat init identityStatus = %q, want \"adopted\"; re-running init re-points at the identity it already made", adopted.IdentityStatus)
	}
	if adopted.Identity != created.Identity {
		t.Fatalf("repeat init identity drifted: %q then %q", created.Identity, adopted.Identity)
	}
	if created.IdentityStatus == adopted.IdentityStatus {
		t.Fatal("created and adopted are not distinguishable in init output")
	}
}

// TestInitWarningNamesTheDifferenceItFiredOn pins that the warning cannot fire
// while describing nothing. Re-pointing a project at a different service state
// changes no field the first draft of this message printed, so it warned with
// both halves identical — the same silent loss this command already had, in the
// mechanism meant to prevent it. Detection and description are now one walk.
func TestInitWarningNamesTheDifferenceItFiredOn(t *testing.T) {
	first, second := onboardingService(t), onboardingService(t)
	target := t.TempDir()
	identity := []string{"--target", target, "--name", "lead", "--role", "workspace-lead"}
	initCommand(t, append([]string{"--state", first}, identity...)...)

	moved, stderr := initCommand(t, append([]string{"--state", second}, identity...)...)
	if moved.Warning == "" {
		t.Fatal("re-pointing the project at a different service state warned about nothing")
	}
	for _, part := range []string{"state", first, second} {
		if !strings.Contains(moved.Warning, part) {
			t.Fatalf("warning %q does not name the changed %q", moved.Warning, part)
		}
	}
	if !strings.Contains(stderr, moved.Warning) {
		t.Fatalf("warning missing from stderr: %q", stderr)
	}
	// It must name only what actually changed: name and role are identical
	// here, and a message claiming otherwise is the defect in reverse.
	for _, absent := range []string{"name", "role", "team", "runtime"} {
		if strings.Contains(moved.Warning, absent+" ") {
			t.Fatalf("warning %q names %q, which did not change", moved.Warning, absent)
		}
	}
}

// TestInitFailureLeavesRecordedDefaultsIntact covers the recorded defaults and
// the announcement about them, and nothing wider. init reads the manifest
// before it creates, enrolls or starts anything, and rewrites it only once the
// enrollment has succeeded, so a run that fails in between leaves the recorded
// defaults byte-identical and says nothing about a replacement that never
// happened — a warning naming defaults still on disk is worse than none.
//
// It does NOT show that such a run leaves the project as it found it, and no
// test here does. Enrollment is not transactional with the manifest write: a
// failure after enrollAgent returns and before WriteFile completes leaves a
// minted session and a connection file behind with no manifest and no report
// naming them, which is the stray-identity class the operator's original
// incident came from. A target that is readable and traversable but not
// writable reaches exactly that state. It predates this change and this change
// does not widen it; see the note at the enrollAgent call in init.go.
//
// The failure injected below is the earliest one after the read, so it proves
// the least while still exercising the ordering truthfully. The interesting
// case is enrollAgent itself failing, and nothing here reaches it.
func TestInitFailureLeavesRecordedDefaultsIntact(t *testing.T) {
	state := onboardingService(t)
	target := t.TempDir()
	first, _ := initCommand(t, "--state", state, "--target", target,
		"--name", "publisher-lead", "--role", "workspace-lead", "--team", "publisher", "--runtime", "claude")
	before, err := os.ReadFile(first.Manifest)
	if err != nil {
		t.Fatal(err)
	}

	// A state path that is a regular file: the directory creation fails at
	// once, after the recorded defaults have been read and well before
	// anything is enrolled or written.
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocked, []byte("occupied"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if err := run(context.Background(), []string{"init", "--state", blocked, "--target", target,
		"--name", "lead", "--role", "workspace-lead"}, nil, &out, &errOut); err == nil {
		t.Fatal("init succeeded with an unusable state directory")
	}

	after, err := os.ReadFile(first.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Fatalf("a failed init still rewrote the recorded defaults: %s", after)
	}
	if out.Len() != 0 || errOut.Len() != 0 {
		t.Fatalf("failed init reported a replacement that never happened: stdout %q stderr %q", out.String(), errOut.String())
	}
}

// TestInitWarnsWhenReplacingDifferingManifest pins the chosen answer to
// "warn or refuse": init proceeds. It replaces a manifest naming different
// defaults and mints a fresh identity for them, and the only thing marking
// that is a warning carried in both the JSON report and on stderr. The
// assertions below record what the operator loses when it fires.
func TestInitWarnsWhenReplacingDifferingManifest(t *testing.T) {
	state := onboardingService(t)
	target := t.TempDir()
	first, quiet := initCommand(t, "--state", state, "--target", target,
		"--name", "publisher-lead", "--role", "workspace-lead", "--team", "publisher", "--runtime", "claude")
	if first.Warning != "" || quiet != "" {
		t.Fatalf("initialising a fresh project warned about nothing: %q / %q", first.Warning, quiet)
	}
	before := readManifest(t, first.Manifest)
	sessionsBefore := len(sessionIDs(t, state))

	// The operator's incident, exactly: bare init in an initialised project.
	replaced, stderr := initCommand(t, "--state", state, "--target", target,
		"--name", "lead", "--role", "workspace-lead")

	if replaced.Warning == "" {
		t.Fatal("init replaced the project defaults with no warning in its JSON")
	}
	for _, part := range []string{"publisher-lead", "publisher", "claude", "lead", "codex"} {
		if !strings.Contains(replaced.Warning, part) {
			t.Fatalf("warning %q does not name both the replaced and the new %q", replaced.Warning, part)
		}
	}
	if !strings.Contains(stderr, replaced.Warning) {
		t.Fatalf("warning missing from stderr, so piping stdout to a parser hides it entirely: %q", stderr)
	}

	// What the warning is warning about. These are losses, not regressions:
	// the manifest's name, team and runtime are overwritten...
	after := readManifest(t, first.Manifest)
	if after == before {
		t.Fatal("manifest unchanged; this test no longer covers the replacement it describes")
	}
	if after.Name != "lead" || after.Team != "" || after.Runtime != "codex" {
		t.Fatalf("manifest = %+v, want the bare-init defaults", after)
	}
	if before.Name != "publisher-lead" || before.Team != "publisher" || before.Runtime != "claude" {
		t.Fatalf("fixture is wrong: %+v", before)
	}
	// ...and a second identity is minted for the same project.
	if got := len(sessionIDs(t, state)); got != sessionsBefore+1 {
		t.Fatalf("expected the replacement to mint one more identity: %d sessions then %d", sessionsBefore, got)
	}
	if replaced.IdentityStatus != "created" || replaced.Identity == first.Identity {
		t.Fatalf("expected a newly minted identity, got %s %q (was %q)", replaced.IdentityStatus, replaced.Identity, first.Identity)
	}
}
