// SPDX-License-Identifier: MPL-2.0

package integration

import (
	"strings"
	"testing"
)

// Every command builds a flag.ContinueOnError FlagSet, points it at the error
// stream and returns whatever Parse returned. flag has already written the
// message and the usage listing by then, so the process boundary printing the
// returned error put the message on stderr a second time -- for all fifteen
// FlagSets, not just doctor's.
//
// Like the unreachable-service double print before it, this is invisible to
// every test that drives run: run returns the error, and only main's boundary
// prints it. So this builds the real binary and reads its stderr.
func TestFlagErrorsAreReportedOnceByTheBuiltBinary(t *testing.T) {
	binary := buildCommonsBinary(t)

	// One argv per FlagSet in cmd/agent-commons. An undefined flag fails Parse
	// before any command does work, so none of these touch a state directory.
	for _, argv := range [][]string{
		{"doctor"},
		{"init"},
		{"watch"},
		{"console"},
		{"update"},
		{"service-plan"},
		{"connect-mcp"},
		{"launch-context"},
		{"wake-maintain"},
		{"wake-resolve"},
		{"enroll"},
		{"check-in"},
		{"codex-prepare"},
		{"codex-resume-check"},
		{"service", "start"},
		{"bundle", "install"},
		// The shared FlagSet in main.go, reached by every command that falls
		// through dispatch.
		{"serve"},
	} {
		name := strings.Join(argv, " ")
		t.Run(name, func(t *testing.T) {
			stdout, stderr, code := runCommons(t, binary, append(argv, "--not-a-real-flag")...)
			if code == 0 {
				t.Fatalf("%s accepted an undefined flag:\n%s", name, stderr)
			}
			const message = "flag provided but not defined: -not-a-real-flag"
			if got := strings.Count(stderr, message); got != 1 {
				t.Fatalf("%s stderr repeats %q %d times, want exactly 1:\n%s", name, message, got, stderr)
			}
			// The duplicate is removed by suppressing the second print, not by
			// silencing flag. flag's half carries the usage listing, which is
			// the half that tells the operator which flags do exist.
			//
			// Naming the header rather than matching any listing is what keeps
			// this row covering the FlagSet it was written for: every one of
			// these headers is its argv, so a command that fell through to the
			// shared FlagSet in main.go would answer under the wrong name here
			// instead of passing on a listing that no longer belongs to it.
			if header := "Usage of " + name + ":"; !strings.Contains(stderr, header) {
				t.Fatalf("%s stderr has no %q listing:\n%s", name, header, stderr)
			}
			if stdout != "" {
				t.Fatalf("%s wrote to stdout: %q", name, stdout)
			}
		})
	}

	// flag.ErrHelp comes back from the same Parse call and is suppressed by the
	// same marker. flag has already written the usage listing an operator asked
	// for; "flag: help requested" underneath it is the boundary talking about
	// its own control flow.
	//
	// "Alone" is the load-bearing word, and asserting the sentinel is absent
	// does not say it. parseFlags returning nil for ErrHelp -- the obvious later
	// "help is not an error" cleanup -- also removes the sentinel, and then the
	// command runs on: doctor prints the listing and its own resolution failure
	// underneath, and update would begin a self-update. So this asserts the
	// shape of the whole stream instead. flag indents every line PrintDefaults
	// writes; anything a command printed afterwards starts at column 0.
	t.Run("help requested prints the usage listing alone", func(t *testing.T) {
		stdout, stderr, code := runCommons(t, binary, "doctor", "-h")
		lines := strings.Split(strings.TrimRight(stderr, "\n"), "\n")
		if lines[0] != "Usage of doctor:" {
			t.Fatalf("doctor -h did not open with the usage listing:\n%s", stderr)
		}
		for _, line := range lines[1:] {
			if line == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
				continue
			}
			t.Fatalf("doctor -h printed %q after the usage listing:\n%s", line, stderr)
		}
		if stdout != "" {
			t.Fatalf("doctor -h wrote to stdout: %q", stdout)
		}
		// Marking the error changes what is printed and nothing else. Both
		// flag_parse.go and main.go state that the exit status is unchanged;
		// this is the only assertion standing behind it.
		if code != 1 {
			t.Fatalf("doctor -h exited %d, want 1", code)
		}
	})
}
