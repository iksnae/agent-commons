// SPDX-License-Identifier: MPL-2.0

package main

import "flag"

// parseFlags parses args with a flag.ContinueOnError FlagSet whose output is
// already the command's error stream, and marks what comes back as reported.
//
// flag writes the message AND the usage listing itself before Parse returns,
// so the process boundary printing the returned error put the message on
// stderr twice. Suppressing the second print rather than silencing flag is
// what keeps the usage listing: flag's half is the half that tells an operator
// which flags do exist, and the boundary's half only ever repeated the first
// line of it.
//
// This is the whole class, not one command: every FlagSet in this package is
// built the same way and returns its parse error the same way, so every one of
// them printed twice. Commands call this instead of fs.Parse.
//
// flag.ErrHelp arrives through the same return and is marked with it. That is
// the intended reading: -h means flag has already written the listing the
// operator asked for, and "flag: help requested" underneath it describes this
// package's control flow rather than anything the operator did wrong. The exit
// status is unchanged in both cases.
func parseFlags(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		return reportedError{err}
	}
	return nil
}
