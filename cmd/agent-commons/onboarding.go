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
	Hold, Once                                                 bool
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
		return enrollAgent(ctx, options, streams.Output)
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
	flags.StringVar(&options.Runtime, "runtime", "", "runtime for this attachment: claude or codex")
	flags.StringVar(&options.NativeSession, "native-session", "", "exact native runtime session ID")
	flags.BoolVar(&options.Hold, "hold", false, "renew attachment while watching for inbox arrivals")
	flags.BoolVar(&options.Once, "once", false, "with hold: exit after one arrival batch")
	if err := flags.Parse(args[1:]); err != nil {
		return options, err
	}
	if flags.NArg() != 0 {
		return options, errors.New("unexpected positional arguments")
	}
	return options, nil
}
