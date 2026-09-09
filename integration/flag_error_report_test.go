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

	// flag.ErrHelp comes back from the same Parse call, and is the outcome the
	// help screen actively sends operators to: help.go writes "Run
	// 'agent-commons COMMAND --help' for flags and examples."
	//
	// It used to answer on stderr with exit 1, which made that instruction
	// hostile in two ordinary settings -- `set -e; agent-commons doctor --help`
	// aborted the script, and `agent-commons doctor --help | less` showed
	// nothing, because the text was on the stream the pipe did not carry. A
	// bare invocation already answers the same question on stdout with exit 0;
	// this is the same gesture and now gets the same treatment.
	//
	// "Alone" is still the load-bearing word, and asserting the sentinel is
	// absent does not say it. parseFlags returning nil for ErrHelp -- the
	// obvious "help is not an error" cleanup -- also removes the sentinel, and
	// then the command runs on: doctor would print the listing and its own
	// resolution failure underneath, and update would begin a self-update. So
	// this asserts the shape of the whole stream instead. flag indents every
	// line PrintDefaults writes; anything a command printed afterwards starts
	// at column 0.
	//
	// Every FlagSet is built by the same parseFlags, so every command answers
	// -h the same way. Running the spelling the help screen prints (--help)
	// alongside -h across several commands is what keeps that true.
	for _, argv := range [][]string{
		{"doctor", "--help"},
		{"doctor", "-h"},
		{"init", "--help"},
		{"watch", "--help"},
		{"enroll", "--help"},
		{"update", "--help"},
		{"bundle", "install", "--help"},
		// The shared FlagSet in main.go, reached by every command that falls
		// through dispatch.
		{"serve", "--help"},
	} {
		name := strings.Join(argv, " ")
		t.Run("help requested: "+name, func(t *testing.T) {
			stdout, stderr, code := runCommons(t, binary, argv...)

			// Exit 0 is the half nobody chose on purpose. It is what makes the
			// instruction on the help screen safe to follow inside `set -e`.
			if code != 0 {
				t.Fatalf("%s exited %d, want 0:\nstdout:\n%s\nstderr:\n%s", name, code, stdout, stderr)
			}
			// Stdout is the half that makes `| less` and `| grep` work. A
			// listing on stderr is invisible to both.
			if stderr != "" {
				t.Fatalf("%s wrote to stderr: %q", name, stderr)
			}
			listing := "Usage of " + strings.Join(argv[:len(argv)-1], " ") + ":"
			lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
			if lines[0] != listing {
				t.Fatalf("%s did not open with %q:\n%s", name, listing, stdout)
			}
			for _, line := range lines[1:] {
				if line == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
					continue
				}
				t.Fatalf("%s printed %q after the usage listing:\n%s", name, line, stdout)
			}
		})
	}
}
