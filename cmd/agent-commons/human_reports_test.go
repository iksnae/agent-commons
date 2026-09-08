// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentcommons/internal/core"
)

// humanCommand runs a command and returns stdout and stderr separately. The two
// streams are never merged here: which one a line lands on is the thing several
// of these tests are about.
func humanCommand(t *testing.T, args ...string) (string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	if err := run(context.Background(), args, nil, &out, &errOut); err != nil {
		t.Fatalf("%v: %v (%s / %s)", args, err, out.String(), errOut.String())
	}
	return out.String(), errOut.String()
}

// enrolledConnection returns a real service state and the connection file path
// of one enrolled role, using the machine document so the fixture does not
// depend on the prose these tests are asserting.
func enrolledConnection(t *testing.T) (string, string) {
	t.Helper()
	state := onboardingService(t)
	data := onboardingCommand(t, "enroll", "--json", "--state", state, "--target", t.TempDir(), "--name", "lead", "--role", "lead")
	var enrollment struct {
		Config string `json:"config"`
	}
	if err := json.Unmarshal(data, &enrollment); err != nil {
		t.Fatal(err)
	}
	return state, enrollment.Config
}

// assertNotJSON fails when a supposedly human rendering would still parse as
// the document it replaced. Asserting on prose alone would pass for output that
// is a JSON document with a friendly notice inside it.
func assertNotJSON(t *testing.T, label, output string) {
	t.Helper()
	var anything any
	if err := json.Unmarshal([]byte(output), &anything); err == nil {
		t.Fatalf("%s still wrote a JSON document by default:\n%s", label, output)
	}
}

// The queue line must not let work stopped by abandonment hide inside a count
// the operator would read as waiting on a membership change.
func TestRuntimeSummaryReportsAbandonedWorkInItsOwnRight(t *testing.T) {
	var out bytes.Buffer
	human := newReport(&out)
	writeRuntimeSummary(human, &core.RuntimeStatusPage{
		Sessions: []core.RuntimeQueueStatus{{
			Identity: "lead", Runtime: "codex",
			Ready: 1, Running: 2, Interrupted: 3, Failed: 4,
			WaitingAbandoned: 5, WaitingTeam: 6,
		}},
	})
	if err := human.write(); err != nil {
		t.Fatal(err)
	}
	rendered := out.String()
	if !strings.Contains(rendered, "5 held by abandoned tasks") {
		t.Fatalf("queue line does not report abandoned work:\n%s", rendered)
	}
	for _, other := range []string{"1 ready", "2 running", "3 interrupted", "4 failed"} {
		if !strings.Contains(rendered, other) {
			t.Fatalf("queue line lost %q:\n%s", other, rendered)
		}
	}
}

func TestInitDefaultsToAHumanSummary(t *testing.T) {
	state := onboardingService(t)
	target := t.TempDir()
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	out, _ := humanCommand(t, "init", "--state", state, "--target", target,
		"--name", "lead", "--role", "workspace-lead", "--runtime", "claude")

	assertNotJSON(t, "init", out)
	// The values an operator has to act on next: where the project is, which
	// identity it got and whether that identity was made or adopted.
	for _, want := range []string{"Project set up", resolved, "claude", "created",
		filepath.Join(resolved, ".agent-commons", "project.json")} {
		if !strings.Contains(out, want) {
			t.Fatalf("init summary omits %q:\n%s", want, out)
		}
	}
}

// The replacement warning stays on stderr in the human form too. It is the only
// notice that a project was re-pointed, and printing it on both streams would
// show it twice on a terminal where both land in the same place.
func TestInitHumanSummaryLeavesTheWarningOnStderr(t *testing.T) {
	first, second := onboardingService(t), onboardingService(t)
	target := t.TempDir()
	identity := []string{"--target", target, "--name", "lead", "--role", "workspace-lead"}
	humanCommand(t, append([]string{"init", "--state", first}, identity...)...)

	out, errOut := humanCommand(t, append([]string{"init", "--state", second}, identity...)...)
	if !strings.Contains(errOut, "replaced existing project defaults") {
		t.Fatalf("the human run lost the replacement warning from stderr: %q", errOut)
	}
	if strings.Contains(out, "replaced existing project defaults") {
		t.Fatalf("the warning was printed on both streams:\n%s", out)
	}
}

func TestEnrollDefaultsToAHumanSummary(t *testing.T) {
	state := onboardingService(t)
	target := t.TempDir()
	args := []string{"enroll", "--state", state, "--target", target, "--name", "lead", "--role", "lead"}

	created, _ := humanCommand(t, args...)
	assertNotJSON(t, "enroll", created)
	if !strings.Contains(created, "Role connected") || !strings.Contains(created, "created") {
		t.Fatalf("enroll summary does not say a new identity was made:\n%s", created)
	}

	// Re-enrolling adopts. That difference is invisible in the identity itself,
	// so the summary has to name it.
	adopted, _ := humanCommand(t, args...)
	if !strings.Contains(adopted, "adopted") {
		t.Fatalf("repeat enroll summary does not say the identity was adopted:\n%s", adopted)
	}
}

// enroll's JSON document is a machine contract: three keys, no more. This pins
// the exact bytes rather than only that they parse, so a field added for a
// human reader cannot leak into the document a parser reads.
func TestEnrollJSONDocumentIsUnchanged(t *testing.T) {
	state := onboardingService(t)
	target := t.TempDir()
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	out, _ := humanCommand(t, "enroll", "--json", "--state", state, "--target", target, "--name", "lead", "--role", "lead")

	var document struct {
		Identity string `json:"identity"`
		Config   string `json:"config"`
		Target   string `json:"target"`
	}
	if err := json.Unmarshal([]byte(out), &document); err != nil {
		t.Fatalf("enroll --json is not machine-readable: %v (%s)", err, out)
	}
	if document.Target != resolved || document.Identity == "" || document.Config == "" {
		t.Fatalf("enroll --json lost a field: %s", out)
	}
	expected, err := json.Marshal(map[string]string{
		"identity": document.Identity, "config": document.Config, "target": document.Target,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out != string(expected)+"\n" {
		t.Fatalf("enroll --json changed shape:\n got %q\nwant %q", out, string(expected)+"\n")
	}
}

func TestDoctorDefaultsToAHumanSummary(t *testing.T) {
	_, config := enrolledConnection(t)
	out, _ := humanCommand(t, "doctor", "--config", config)

	assertNotJSON(t, "doctor", out)
	// The notice is carried into the human form too: "ready" means the scoped
	// RPC checks passed, and the summary must not let that read as more.
	for _, want := range []string{"Connection ready", "inbox.page", "board.list", "Read-only connection check"} {
		if !strings.Contains(out, want) {
			t.Fatalf("doctor summary omits %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Welcome to Agent Commons") {
		t.Fatal("doctor summary printed peer message text")
	}
}

// A failing doctor has to name the check that failed and its stable code in the
// human form too. Reporting only "not ready" would leave the operator with less
// than the JSON document gave them.
func TestDoctorHumanSummaryNamesTheFailingCheck(t *testing.T) {
	var out, errOut bytes.Buffer
	missing := filepath.Join(t.TempDir(), "missing")
	if err := run(context.Background(), []string{"doctor", "--config", missing}, nil, &out, &errOut); err == nil {
		t.Fatal("doctor reported a missing connection as healthy")
	}
	assertNotJSON(t, "doctor", out.String())
	for _, want := range []string{"Connection not ready", "not_found", "private-connection-and-credential"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("doctor summary omits %q:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), missing) {
		t.Fatalf("doctor summary exposed the path it was given:\n%s", out.String())
	}
}

func TestBundleDefaultsToAHumanSummaryForEveryOperation(t *testing.T) {
	source, installed := stagedBundle(t), filepath.Join(t.TempDir(), "installed")

	install, _ := humanCommand(t, "bundle", "install", "--from", source, "--to", installed)
	assertNotJSON(t, "bundle install", install)
	for _, want := range []string{"Bundle installed", installed, filepath.Join(installed, "agent-commons")} {
		if !strings.Contains(install, want) {
			t.Fatalf("bundle install summary omits %q:\n%s", want, install)
		}
	}

	verify, _ := humanCommand(t, "bundle", "verify", "--to", installed)
	assertNotJSON(t, "bundle verify", verify)
	if !strings.Contains(verify, "Bundle verified") || !strings.Contains(verify, "not a signature verification") {
		t.Fatalf("bundle verify summary is missing its result or its limit:\n%s", verify)
	}

	remove, _ := humanCommand(t, "bundle", "remove", "--to", installed, "--confirm-stopped")
	assertNotJSON(t, "bundle remove", remove)
	if !strings.Contains(remove, "Bundle removed") || !strings.Contains(remove, "Moved to") {
		t.Fatalf("bundle remove summary does not say where the installation went:\n%s", remove)
	}
}

// --json applies to all three operations from the one flag, not per subcommand.
func TestBundleJSONAppliesToEveryOperation(t *testing.T) {
	source, installed := stagedBundle(t), filepath.Join(t.TempDir(), "installed")
	for _, args := range [][]string{
		{"bundle", "install", "--json", "--from", source, "--to", installed},
		{"bundle", "verify", "--json", "--to", installed},
		{"bundle", "remove", "--json", "--to", installed, "--confirm-stopped"},
	} {
		out, _ := humanCommand(t, args...)
		var document map[string]string
		if err := json.Unmarshal([]byte(out), &document); err != nil {
			t.Fatalf("%v is not machine-readable: %v (%s)", args, err, out)
		}
		if document["operation"] != args[1] || document["directory"] != installed || document["notice"] == "" {
			t.Fatalf("%v lost a document field: %s", args, out)
		}
	}
}

// stagedBundle lays out the minimum an install accepts: every file the
// installer requires, with stand-in contents. It stages a bundle rather than
// unpacking a real archive because these tests are about which form the report
// takes, not about what a distributable archive contains.
func stagedBundle(t *testing.T) string {
	t.Helper()
	source := t.TempDir()
	for _, name := range []string{"agent-commons", "LICENSE", "LICENSING.md", "INSTALL.md",
		"README.md", "source.tar.gz", "third-party-notices/go/LICENSE"} {
		path := filepath.Join(source, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("stand-in for "+name+"\n"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	return source
}

// check-in's stdout is exactly one JSON document and nothing else. The proof is
// byte-level: the document is decoded and re-encoded, and the result must equal
// the whole of stdout. A header prepended, appended or interleaved there would
// fail this even though the document itself still parses.
func TestCheckInHeaderLeavesStdoutByteIdentical(t *testing.T) {
	_, config := enrolledConnection(t)
	var out, errOut bytes.Buffer
	args := []string{"check-in", "--config", config, "--runtime", "claude", "--native-session", "native-header"}
	if err := run(context.Background(), args, nil, &out, &errOut); err != nil {
		t.Fatal(err)
	}

	var snapshot checkInSnapshot
	if err := json.Unmarshal(out.Bytes(), &snapshot); err != nil {
		t.Fatalf("check-in stdout is not one machine-readable document: %v (%s)", err, out.String())
	}
	var document bytes.Buffer
	if err := json.NewEncoder(&document).Encode(snapshot); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out.Bytes(), document.Bytes()) {
		t.Fatalf("check-in stdout is not exactly the snapshot document:\n got %q\nwant %q", out.String(), document.String())
	}

	// The header exists, and it is on the other stream.
	if !strings.Contains(errOut.String(), "Checked in") {
		t.Fatalf("no human header reached stderr: %q", errOut.String())
	}
	// Strings unique to the header. The snapshot carries its own "peer data,
	// not authority" notice, so that phrase cannot distinguish the two.
	for _, header := range []string{"Checked in", "Attached", "no team was joined"} {
		if strings.Contains(out.String(), header) {
			t.Fatalf("header text %q reached stdout:\n%s", header, out.String())
		}
	}
}

// The header names the role and what it attached to, because that is what the
// human caller documented at plugins/agent-commons/scripts/check-in.sh gets
// instead of the snapshot, which is peer data they should not have to read.
func TestCheckInHeaderNamesTheRoleAndAttachment(t *testing.T) {
	_, config := enrolledConnection(t)
	var out, errOut bytes.Buffer
	args := []string{"check-in", "--config", config, "--runtime", "claude", "--native-session", "native-named"}
	if err := run(context.Background(), args, nil, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"lead", "claude session native-named"} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("check-in header omits %q:\n%s", want, errOut.String())
		}
	}
}

// check-in must not acquire a --json flag: its stdout has only one form, so the
// flag would be a no-op that reads like a promise.
func TestCheckInHasNoJSONFlag(t *testing.T) {
	_, config := enrolledConnection(t)
	args := []string{"check-in", "--json", "--config", config, "--runtime", "claude", "--native-session", "native-flag"}
	if err := run(context.Background(), args, nil, io.Discard, io.Discard); err == nil {
		t.Fatal("check-in accepted --json; its stdout has no second form to select")
	}
}
