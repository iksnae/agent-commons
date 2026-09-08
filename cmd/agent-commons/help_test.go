// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/term"
)

// renderHelp returns the help screen written to an in-memory buffer, which is
// never a terminal and therefore always the plain rendering.
func renderHelp(t *testing.T) string {
	t.Helper()
	var out bytes.Buffer
	if err := writeHelp(&out); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

// A caller who runs the binary with no arguments is told to see help; help has
// to answer the question that sends them there, which is how a command is
// spelled at the shell.
func TestHelpShowsTheInvocationSynopsis(t *testing.T) {
	help := renderHelp(t)
	if !strings.Contains(help, "agent-commons <command> [flags]") {
		t.Fatalf("help omits the usage synopsis:\n%s", help)
	}
}

// help and --version read one variable, so they cannot disagree about which
// release the operator is holding. The stamped value is substituted here so a
// hard-coded literal that happens to match the unstamped default still fails.
func TestHelpTitleCarriesTheBuiltVersion(t *testing.T) {
	built := version
	t.Cleanup(func() { version = built })
	version = "v9.9.9-test-stamp"

	help := renderHelp(t)
	want := "Agent Commons " + version
	first, _, _ := strings.Cut(help, "\n")
	if first != want {
		t.Fatalf("help title is %q, want %q", first, want)
	}
}

// init and enroll read almost identically until today; the difference that
// costs an operator an accidental identity is that only init writes project
// defaults, so both halves of that contrast are asserted here.
func TestHelpDistinguishesInitFromEnroll(t *testing.T) {
	help := renderHelp(t)
	for _, want := range []string{
		"set up a project: enroll or adopt a role, write project defaults",
		"create or adopt a project + role connection, no project defaults",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("help omits %q:\n%s", want, help)
		}
	}
}

// call and methods exist for agents and scripts, not for the operator reading
// this screen; the marker is row text in the one catalog every rendering reads.
func TestHelpMarksTheAgentFacingCommands(t *testing.T) {
	help := renderHelp(t)
	for _, want := range []string{
		"invoke one RPC method (agents)",
		"print the RPC catalog (agents)",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("help omits %q:\n%s", want, help)
		}
	}
}

// The new-operator path is named on the screen, not left to be inferred from
// the command list.
func TestHelpPointsANewOperatorAtInitThenDoctor(t *testing.T) {
	help := renderHelp(t)
	if !strings.Contains(help, startHere) {
		t.Fatalf("help omits the getting-started line:\n%s", help)
	}
	if !strings.Contains(startHere, "init") || !strings.Contains(startHere, "doctor") {
		t.Fatalf("getting-started line names neither init nor doctor: %q", startHere)
	}
	connect := strings.Index(help, "Connect a session")
	hint := strings.Index(help, startHere)
	service := strings.Index(help, "Run the local service")
	if connect < 0 || hint < connect || hint > service {
		t.Fatalf("getting-started line is not inside the Connect a session block:\n%s", help)
	}
}

// Styling is for a human at a terminal. A redirected file, a pipe or a captured
// buffer must receive text a downstream reader can use unchanged.
func TestHelpIsPlainWhenOutputIsNotATerminal(t *testing.T) {
	if escaped := renderHelp(t); strings.Contains(escaped, "\x1b") {
		t.Fatalf("buffered help carries escape sequences: %q", escaped)
	}

	path := filepath.Join(t.TempDir(), "help.txt")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if term.IsTerminal(file.Fd()) {
		t.Fatalf("temporary file %s reports as a terminal; the probe is broken", path)
	}
	if err := writeHelp(file); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(written, []byte{0x1b}) {
		t.Fatalf("redirected help carries escape sequences: %q", written)
	}
	if !bytes.Contains(written, []byte("agent-commons <command> [flags]")) {
		t.Fatalf("redirected help lost its content: %q", written)
	}
}
