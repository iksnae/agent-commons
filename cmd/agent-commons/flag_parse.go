// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"errors"
	"flag"
	"io"
)

// parseFlags parses args with a flag.ContinueOnError FlagSet and routes what
// flag wrote by which of the two outcomes actually happened.
//
// flag writes the message AND the usage listing itself before Parse returns, so
// the process boundary printing the returned error put the message on stderr
// twice. Suppressing the second print rather than silencing flag is what keeps
// the usage listing: flag's half is the half that tells an operator which flags
// do exist, and the boundary's half only ever repeated the first line of it.
//
// This is the whole class, not one command: every FlagSet in this package is
// built the same way and returns its parse error the same way, so every one of
// them printed twice. Commands call this instead of fs.Parse.
//
// The two outcomes reach different streams, which is why flag's output is
// captured here instead of pointed straight at one of them. flag cannot make
// the distinction: it renders the same listing through the same writer whether
// the operator ASKED for it with -h or TRIPPED it with a bad flag. Only the
// error it returns says which, and that is known only after Parse.
//
//   - flag.ErrHelp is a satisfied request. The listing is the answer, so it goes
//     to the OUTPUT stream and the boundary exits 0 -- matching a bare
//     invocation, which answers the same question the same way. `COMMAND --help`
//     is what the help screen tells operators to run; it must not abort a
//     `set -e` script, and it must survive a pipe into a pager.
//   - a genuine parse failure is a diagnosis. The message and the listing stay
//     on the ERROR stream, exit stays non-zero, and reportedError keeps the
//     boundary from printing the message underneath a second time.
func parseFlags(fs *flag.FlagSet, args []string, out io.Writer) error {
	errOut := fs.Output()
	// Capturing and replaying costs one buffer per parse and is what lets the
	// stream be chosen after the fact. Restoring the writer immediately keeps
	// this local: a FlagSet that outlives the call (fs.Args() readers, and the
	// shared set in main.go) is handed back exactly as it arrived.
	var listing bytes.Buffer
	fs.SetOutput(&listing)
	err := fs.Parse(args)
	fs.SetOutput(errOut)

	if err == nil {
		return nil
	}
	if errors.Is(err, flag.ErrHelp) {
		if _, writeErr := listing.WriteTo(out); writeErr != nil {
			return writeErr
		}
		return helpRequestedError{}
	}
	if _, writeErr := listing.WriteTo(errOut); writeErr != nil {
		return writeErr
	}
	return reportedError{err}
}

// helpRequestedError reports that the operator asked for a command's usage
// listing and got it. It is not a failure, and the listing has already been
// written to the output stream, so the process boundary exits 0 and prints
// nothing underneath.
//
// It is a distinct type rather than flag.ErrHelp itself so that only a listing
// this package actually rendered can produce a zero exit. Error() is still a
// real sentence because errors.As is what reads this, never a human.
type helpRequestedError struct{}

func (helpRequestedError) Error() string { return "help requested" }
