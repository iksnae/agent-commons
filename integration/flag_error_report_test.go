// SPDX-License-Identifier: MPL-2.0

package integration

import (
	"encoding/json"
	"slices"
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
		{"service", "status", "--help"},
		// The three that reach no FlagSet at all. bundle and service consume
		// args[0] as a subcommand name, so the flag never reaches a Parse call
		// -- bundle answered a missing --to and service answered an unknown
		// operation. harnesses builds no FlagSet in the first place and
		// rejected the flag as an argument. All three exited 1 with nothing on
		// stdout, which is the shape this row set exists to forbid.
		{"bundle", "--help"},
		{"bundle", "-h"},
		{"service", "--help"},
		{"service", "-h"},
		{"harnesses", "--help"},
		{"harnesses", "-h"},
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

// bundle and service dispatch a subcommand AND carry flags, and the operator
// running `bundle --help` has asked about both halves. Neither half can be
// answered by the subcommand FlagSet the old code built: it is named for a
// subcommand the operator has not chosen yet.
//
// So both halves are asserted from the command itself rather than from a
// transcription. The subcommand names have to be ones the command accepts --
// naming a set nobody can run is the failure mode of a hand-written usage
// string, and `usage: bundle install|verify|remove --to DIR [--from DIR]
// [--confirm-stopped]` had already drifted past --json without anyone noticing.
// The flags have to be exactly the flags a subcommand form lists, which is what
// says the top-level listing is built from the same registration rather than
// from a second copy of it that can drift the same way.
func TestDispatchingCommandsAnswerHelpForBothHalves(t *testing.T) {
	binary := buildCommonsBinary(t)

	for _, command := range []struct {
		name        string
		subcommands []string
		// flagSource is a subcommand form of the same command. Its listing is
		// PrintDefaults over the same flags, so comparing against it is what
		// makes "the top-level listing shows the real flags" checkable without
		// naming a single flag here.
		flagSource []string
	}{
		{
			name:        "bundle",
			subcommands: []string{"install", "verify", "remove"},
			flagSource:  []string{"bundle", "install", "--help"},
		},
		{
			name:        "service",
			subcommands: []string{"install", "enable", "start", "status", "stop", "remove"},
			flagSource:  []string{"service", "status", "--help"},
		},
	} {
		t.Run(command.name, func(t *testing.T) {
			listing, stderr, code := runCommons(t, binary, command.name, "--help")
			if code != 0 || stderr != "" {
				t.Fatalf("%s --help exited %d with stderr %q", command.name, code, stderr)
			}

			// The whole alternation, not each name in turn. Searching for the
			// names one at a time asserts less than it reads: "stop" is inside
			// "-confirm-stopped" and "install" is inside "(install only)", so
			// dropping either subcommand from the synopsis would still be found
			// somewhere in the flag descriptions underneath.
			alternation := strings.Join(command.subcommands, "|")
			if !strings.Contains(listing, alternation) {
				t.Fatalf("%s --help does not offer %q:\n%s", command.name, alternation, listing)
			}

			for _, sub := range command.subcommands {
				// Naming it is worth something only if it is a form that runs.
				subOut, subErr, subCode := runCommons(t, binary, command.name, sub, "--help")
				if subCode != 0 {
					t.Fatalf("%s --help names %q, but `%s %s --help` exited %d:\nstdout:\n%s\nstderr:\n%s",
						command.name, sub, command.name, sub, subCode, subOut, subErr)
				}
				if subErr != "" {
					t.Fatalf("%s %s --help wrote to stderr: %q", command.name, sub, subErr)
				}
			}

			source, _, sourceCode := runCommons(t, binary, command.flagSource...)
			if sourceCode != 0 {
				t.Fatalf("%v exited %d", command.flagSource, sourceCode)
			}
			want := flagNames(source)
			// Positive control on the reader. An empty want makes the
			// comparison below assert nothing, and would then pass against a
			// top-level listing carrying no flags at all.
			if len(want) == 0 {
				t.Fatalf("read no flag names out of %v; the reader is broken, not the listing:\n%s", command.flagSource, source)
			}
			if got := flagNames(listing); !slices.Equal(got, want) {
				t.Fatalf("%s --help lists flags %v, but %v lists %v:\n%s", command.name, got, command.flagSource, want, listing)
			}
		})
	}
}

// harnesses has no flags and no subcommands, so the only thing its listing can
// be wrong about is whether it is a listing at all. This command's stdout is a
// machine contract -- the harness catalog as JSON -- and the one answer to
// --help worse than exiting 1 is running anyway and handing the catalog to a
// reader who asked what the command does.
func TestHarnessesAnswersHelpInsteadOfRunning(t *testing.T) {
	binary := buildCommonsBinary(t)

	help, stderr, code := runCommons(t, binary, "harnesses", "--help")
	if code != 0 || stderr != "" {
		t.Fatalf("harnesses --help exited %d with stderr %q", code, stderr)
	}
	if !strings.Contains(help, "agent-commons harnesses") {
		t.Fatalf("harnesses --help does not show the invocation it documents:\n%s", help)
	}

	catalog, catalogErr, catalogCode := runCommons(t, binary, "harnesses")
	if catalogCode != 0 {
		t.Fatalf("harnesses exited %d: %s", catalogCode, catalogErr)
	}
	if help == catalog {
		t.Fatalf("harnesses --help printed the catalog rather than a usage listing:\n%s", help)
	}
	if json.Valid([]byte(help)) {
		t.Fatalf("harnesses --help printed machine output where a listing belongs:\n%s", help)
	}
}

// The three commands above now read two spellings as a request for help. That
// is a narrower change than "a leading dash means help", and the difference is
// visible only from the cases that must stay diagnoses: a subcommand nobody
// defined, an argument a command takes none of, and a flag that does not exist.
//
// Byte-exact, because what is being held is the message an operator reads and a
// script matches, not merely that something failed.
func TestBadSubcommandsAndArgumentsStayErrors(t *testing.T) {
	binary := buildCommonsBinary(t)

	for _, want := range []struct {
		argv   []string
		stderr string
	}{
		{[]string{"bundle", "bogus"}, "bundle requires --to and no positional arguments\n"},
		{[]string{"service", "bogus"}, "unknown service operation: bogus\n"},
		{[]string{"harnesses", "extra-arg"}, "harnesses takes no arguments\n"},
		{[]string{"bundle", "--not-a-real-flag"}, "bundle requires --to and no positional arguments\n"},
		{[]string{"service", "--not-a-real-flag"}, "unknown service operation: --not-a-real-flag\n"},
		{[]string{"harnesses", "--not-a-real-flag"}, "harnesses takes no arguments\n"},
	} {
		name := strings.Join(want.argv, " ")
		t.Run(name, func(t *testing.T) {
			stdout, stderr, code := runCommons(t, binary, want.argv...)
			if code != 1 {
				t.Fatalf("%s exited %d, want 1:\nstdout:\n%s\nstderr:\n%s", name, code, stdout, stderr)
			}
			if stdout != "" {
				t.Fatalf("%s wrote a diagnosis to stdout: %q", name, stdout)
			}
			if stderr != want.stderr {
				t.Fatalf("%s stderr = %q, want %q", name, stderr, want.stderr)
			}
		})
	}
}

// flagNames reads the flag names out of whatever PrintDefaults wrote. Every
// entry opens a line with two spaces and a dash, and the name runs to the first
// space, which separates it from its type word. Indented lines that are not
// entries -- a synopsis, a subcommand description, the second line of an entry
// -- do not open with a dash and are skipped.
func flagNames(listing string) []string {
	var names []string
	for _, line := range strings.Split(listing, "\n") {
		if !strings.HasPrefix(line, "  -") {
			continue
		}
		name := strings.TrimPrefix(line, "  ")
		if cut := strings.IndexByte(name, ' '); cut >= 0 {
			name = name[:cut]
		}
		names = append(names, name)
	}
	return names
}
