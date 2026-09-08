// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
)

type onboardingOptions struct {
	Command, State, Config, Identity, Name, Role, Team, Target string
	Runtime, NativeSession                                     string
	LaunchDirectory                                            string
	Hold, Once                                                 bool
	// JSON is set only by enroll. check-in has no such flag: its stdout is one
	// JSON document unconditionally, so there is no second form to select.
	JSON bool
}

type commandStreams struct {
	Output io.Writer
	Errors io.Writer
}

func runOnboarding(ctx context.Context, args []string, out, errOut io.Writer) error {
	options, err := parseOnboarding(args, errOut)
	if err != nil {
		return err
	}
	streams := commandStreams{Output: out, Errors: errOut}
	if options.Command == "enroll" {
		enrolled, err := enrollAgent(ctx, options)
		if err != nil {
			return err
		}
		if options.JSON {
			return writeEnrollmentDocument(streams.Output, enrolled)
		}
		return writeEnrollmentSummary(streams.Output, enrolled)
	}
	return checkInAgent(ctx, options, streams)
}

func parseOnboarding(args []string, errorsOut io.Writer) (onboardingOptions, error) {
	options := onboardingOptions{Command: args[0]}
	home, err := os.UserHomeDir()
	if err != nil {
		return options, err
	}
	flags := flag.NewFlagSet(options.Command, flag.ContinueOnError)
	flags.SetOutput(errorsOut)
	flags.StringVar(&options.State, "state", filepath.Join(home, ".local", "state", "agent-commons"), "private service state directory")
	flags.StringVar(&options.Config, "config", "", "private role connection configuration")
	flags.StringVar(&options.Identity, "id", "", "existing identity ID; otherwise derived from project + name + role")
	flags.StringVar(&options.Name, "name", "", "stable project agent name")
	flags.StringVar(&options.Role, "role", "", "project role")
	flags.StringVar(&options.Team, "team", "", "project team label")
	flags.StringVar(&options.Target, "target", "", "canonical project/workspace directory")
	flags.StringVar(&options.Runtime, "runtime", "", "runtime for this attachment: "+runtimeVocabulary())
	flags.StringVar(&options.NativeSession, "native-session", "", "exact native runtime session ID")
	flags.StringVar(&options.LaunchDirectory, "launch-directory", "", "native launch directory; must be the enrolled target or a directory beneath it")
	flags.BoolVar(&options.Hold, "hold", false, "renew attachment while watching for inbox arrivals")
	flags.BoolVar(&options.Once, "once", false, "with hold: exit after one arrival batch")
	// Registered for enroll only, so `check-in --json` is an unknown flag
	// rather than a silently accepted no-op. check-in's stdout is a machine
	// contract with no alternative form; its human header goes to stderr.
	if options.Command == "enroll" {
		flags.BoolVar(&options.JSON, "json", false, jsonFlagUsage)
	}
	if err := flags.Parse(args[1:]); err != nil {
		return options, err
	}
	if flags.NArg() != 0 {
		return options, errors.New("unexpected positional arguments")
	}
	return options, nil
}
