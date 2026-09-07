// SPDX-License-Identifier: MPL-2.0

package main

import (
	"errors"
	"flag"
	"io"
)

type wakeResolveOptions struct {
	Config, Thread, Message, Expected, Decision, Evidence, OperatorToken string
	Confirmed                                                            bool
}

func parseWakeResolution(args []string, errOut io.Writer) (wakeResolveOptions, error) {
	var o wakeResolveOptions
	f := flag.NewFlagSet("wake-resolve", flag.ContinueOnError)
	f.SetOutput(errOut)
	f.StringVar(&o.Config, "config", "", "private enrolled-role connection")
	f.StringVar(&o.Thread, "codex-thread", "", "exact thread UUID")
	f.StringVar(&o.Message, "message-id", "", "exact notification message ID")
	f.StringVar(&o.Expected, "expected-hash", "", "record hash returned by inspection")
	f.StringVar(&o.Decision, "decision", "", "retry or suppress; omitted inspects without changing the ledger")
	f.StringVar(&o.Evidence, "evidence", "", "operator reasoning, at most 4096 bytes; do not include secrets")
	f.StringVar(&o.OperatorToken, "operator-token-file", "", "explicit private operator credential for applying a decision")
	f.BoolVar(&o.Confirmed, "acknowledge-notification-risk", false, "acknowledge possible duplicate notification or suppression without delivery")
	if err := f.Parse(args); err != nil {
		return o, err
	}
	if o.Config == "" || o.Thread == "" || o.Message == "" || len(o.Message) > 256 || f.NArg() != 0 {
		return o, errors.New("wake-resolve requires --config, --codex-thread and --message-id")
	}
	if o.Decision == "" {
		if o.Expected != "" || o.Evidence != "" || o.OperatorToken != "" || o.Confirmed {
			return o, errors.New("inspection takes no decision flags")
		}
	} else if o.Expected == "" || o.Evidence == "" || o.OperatorToken == "" || !o.Confirmed {
		return o, errors.New("applying a decision requires --expected-hash, --evidence, --operator-token-file and --acknowledge-notification-risk")
	}
	return o, nil
}
