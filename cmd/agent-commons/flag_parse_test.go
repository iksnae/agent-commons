// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"flag"
	"testing"
)

// parseFlags points the FlagSet at a buffer of its own so it can choose the
// stream after Parse has said which outcome happened, then points it back.
//
// The restore is what this pins. Without it the FlagSet is left addressing a
// bytes.Buffer that has gone out of scope, so a later fs.Usage() or
// fs.PrintDefaults() writes a diagnosis into a dead buffer and it is silently
// lost -- the failure mode that leaves no trace anywhere to find it by.
//
// Nothing in the package does that today: the shared FlagSet in main.go is
// only read for values after parsing, and the per-command sets are discarded.
// So this is a latent invariant with no live caller, which is exactly the kind
// of claim that rots unpinned -- the comment would go on asserting the writer
// was "handed back exactly as it arrived" long after it was not.
//
// All three outcomes are covered because each takes a different path out of
// parseFlags, and only one of them is on the branch a reader would think to
// check.
func TestParseFlagsRestoresTheFlagSetWriter(t *testing.T) {
	for _, outcome := range []struct {
		name string
		args []string
	}{
		{"help requested", []string{"-h"}},
		{"parse failure", []string{"--not-a-real-flag"}},
		{"clean parse", []string{"--real", "value"}},
	} {
		t.Run(outcome.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			fs := flag.NewFlagSet("probe", flag.ContinueOnError)
			fs.SetOutput(&errOut)
			fs.String("real", "", "a real flag")
			_ = parseFlags(fs, outcome.args, &out)

			// Whatever parseFlags already routed is not what is being measured;
			// only where the NEXT write lands is.
			out.Reset()
			errOut.Reset()
			fs.PrintDefaults()

			if errOut.Len() == 0 {
				t.Fatal("PrintDefaults wrote nothing to the FlagSet's original writer; parseFlags left it addressing its internal buffer")
			}
			if out.Len() != 0 {
				t.Fatalf("PrintDefaults wrote to the output stream: %q", out.String())
			}
		})
	}
}
