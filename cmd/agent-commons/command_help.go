// SPDX-License-Identifier: MPL-2.0

package main

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

// helpRequested reports whether the operator asked a command for its usage
// listing before choosing a subcommand.
//
// parseFlags answers -h for every form that reaches a FlagSet, which is most of
// them. It cannot answer for a command whose first argument is a subcommand
// slot: bundle and service spend args[0] on the subcommand name and hand
// args[1:] to Parse, so `bundle --help` builds a FlagSet called "bundle --help"
// and parses nothing. flag never sees the flag, and the operator gets whichever
// check happens to fail next -- a missing --to, or an unknown operation.
// harnesses reaches no FlagSet at all.
//
// Only the first argument, and only these two spellings. Once a subcommand IS
// chosen the form reaches parseFlags, where flag answers -h wherever it appears
// and rejects every other undefined flag; widening this would take those cases
// away from flag and turn `bundle --not-a-real-flag` into a help screen.
func helpRequested(args []string) bool {
	return len(args) > 0 && (args[0] == "-h" || args[0] == "--help")
}

// flaglessCommand names a command that parses no flags at all, so that it can
// be rendered by writeCommandHelp like any other. PrintDefaults over an empty
// FlagSet writes nothing, which is what a command with no flags should show.
func flaglessCommand(name string) *flag.FlagSet {
	return flag.NewFlagSet(name, flag.ContinueOnError)
}

// writeCommandHelp renders the listing for a command that dispatches
// subcommands, and returns the same sentinel parseFlags returns for -h: the
// listing has been written to the output stream, so the process boundary exits
// 0 and prints nothing underneath it.
//
// The flags come from fs rather than from a sentence, because a hand-written
// flag list is the thing that drifts. bundle's usage string still reads
// `--to DIR [--from DIR] [--confirm-stopped]` after --json was added to the
// FlagSet beside them. PrintDefaults over the FlagSet the command actually
// parses cannot fall behind that way -- so the synopsis names the subcommand
// slot and says [flags], and the flags speak for themselves underneath.
//
// The shape matches what flag itself writes for a subcommand form: a "Usage of
// NAME:" header, then indented lines and nothing else. `bundle --help` and
// `bundle install --help` read as the same kind of screen because they are.
func writeCommandHelp(out io.Writer, fs *flag.FlagSet, synopsis string, detail []string) error {
	var screen strings.Builder
	fmt.Fprintf(&screen, "Usage of %s:\n  %s\n", fs.Name(), synopsis)
	for _, line := range detail {
		if line == "" {
			screen.WriteString("\n")
			continue
		}
		fmt.Fprintf(&screen, "  %s\n", line)
	}

	defined := false
	fs.VisitAll(func(*flag.Flag) { defined = true })
	if defined {
		screen.WriteString("\n")
	}
	if _, err := io.WriteString(out, screen.String()); err != nil {
		return err
	}

	// PrintDefaults writes nothing at all for a FlagSet with no flags, which is
	// why the blank line above is conditional: harnesses would otherwise end on
	// a stray one.
	previous := fs.Output()
	fs.SetOutput(out)
	fs.PrintDefaults()
	fs.SetOutput(previous)
	return helpRequestedError{}
}
