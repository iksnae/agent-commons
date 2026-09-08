// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/x/term"
)

// NO_COLOR must win even when the destination really is a terminal. The
// descriptor half of that decision cannot be faked without a pseudo-terminal,
// so the policy is asserted directly and both refusals are covered. This now
// gates every human rendering in the CLI, not only the help screen.
func TestNoColorDisablesStylingOnATerminal(t *testing.T) {
	if !stylingAllowed(true, "") {
		t.Fatal("a terminal with NO_COLOR unset should be styled; the probe cannot detect a regression")
	}
	if stylingAllowed(true, "1") {
		t.Fatal("NO_COLOR=1 did not disable styling on a terminal")
	}
	if stylingAllowed(false, "") {
		t.Fatal("a non-terminal was styled")
	}
}

// The same gate is exercised end to end against a real terminal where the test
// run has one; it is skipped, not faked, where it does not.
func TestNoColorDisablesStylingOnTheControllingTerminal(t *testing.T) {
	tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		t.Skipf("no controlling terminal to test the NO_COLOR gate against: %v", err)
	}
	defer tty.Close()
	if !term.IsTerminal(tty.Fd()) {
		t.Skip("/dev/tty is not reported as a terminal here")
	}

	t.Setenv("NO_COLOR", "")
	os.Unsetenv("NO_COLOR")
	if !writerIsStyled(tty) {
		t.Fatal("styling is off for a terminal with NO_COLOR unset; the probe cannot detect the gate")
	}
	t.Setenv("NO_COLOR", "1")
	if writerIsStyled(tty) {
		t.Fatal("NO_COLOR=1 did not disable styling on a terminal")
	}
}

// A buffer is never a terminal, so every block a command writes into one must
// be plain text a downstream reader can use unchanged.
func TestReportBlocksArePlainWhenTheDestinationIsNotATerminal(t *testing.T) {
	var out bytes.Buffer
	human := newReport(&out)
	human.headline("Headline")
	human.field("Label", "value")
	human.note("aside")
	if err := human.write(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "\x1b") {
		t.Fatalf("buffered report carries escape sequences: %q", out.String())
	}
}

// Field labels are padded to a common width so values line up. The padding is
// measured on the label, and a renderer that pads by a constant instead would
// misalign the moment two labels differ in length.
func TestReportAlignsFieldValuesOnTheLongestLabel(t *testing.T) {
	var out bytes.Buffer
	human := newReport(&out)
	human.field("Project", "/one")
	human.field("Service state", "/two")
	if err := human.write(); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected two field lines, got %q", out.String())
	}
	if strings.Index(lines[0], "/one") != strings.Index(lines[1], "/two") {
		t.Fatalf("field values are not aligned:\n%s", out.String())
	}
}
