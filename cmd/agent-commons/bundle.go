// SPDX-License-Identifier: MPL-2.0

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"path/filepath"

	"agentcommons/internal/installation"
)

// bundleOperations is the alternation both the usage sentence and the help
// synopsis name, written once so the two cannot come to disagree about which
// subcommands exist.
const bundleOperations = "install|verify|remove"

// bundleOptions is what bundle parses. Registration lives in one function
// because the listing `bundle --help` prints is PrintDefaults over the same
// FlagSet the subcommand forms parse -- a second registration for the help
// screen would be a second thing to keep in step, which is the failure this
// screen exists to end.
type bundleOptions struct {
	source  string
	target  string
	stopped bool
	asJSON  bool
}

func bundleFlagSet(name string, errOut io.Writer) (*flag.FlagSet, *bundleOptions) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(errOut)
	var options bundleOptions
	fs.StringVar(&options.source, "from", "", "extracted native archive directory")
	fs.StringVar(&options.target, "to", "", "absolute dedicated installation directory")
	fs.BoolVar(&options.stopped, "confirm-stopped", false, "confirm all services using this installation have been stopped before removal")
	// One flag on the one FlagSet: --json means the same thing for install,
	// verify and remove, and is not something a caller has to look up per
	// subcommand.
	fs.BoolVar(&options.asJSON, "json", false, jsonFlagUsage)
	return fs, &options
}

func runBundle(args []string, out, errOut io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: bundle " + bundleOperations + " --to DIR [--from DIR] [--confirm-stopped]")
	}
	// Asked before args[0] is spent on a subcommand name: this is the one point
	// where `bundle --help` and `bundle install --help` differ, and answering
	// it here is what stops the flag from being read as a subcommand.
	if helpRequested(args) {
		fs, _ := bundleFlagSet("bundle", errOut)
		return writeCommandHelp(out, fs, "agent-commons bundle "+bundleOperations+" [flags]", []string{
			"",
			"install   copy an extracted native archive into --to and record a receipt",
			"verify    check the installed files against that receipt",
			"remove    move an installation aside, keeping service state and agents",
		})
	}
	fs, options := bundleFlagSet("bundle "+args[0], errOut)
	if err := parseFlags(fs, args[1:], out); err != nil {
		return err
	}
	if fs.NArg() != 0 || options.target == "" {
		return errors.New("bundle requires --to and no positional arguments")
	}
	result := map[string]string{"operation": args[0], "directory": options.target}
	switch args[0] {
	case "install":
		if options.source == "" || options.stopped {
			return errors.New("install requires --from; --confirm-stopped is only for remove")
		}
		if err := installation.Install(options.source, options.target); err != nil {
			return err
		}
		result["binary"] = filepath.Join(options.target, "agent-commons")
		result["notice"] = "Local bundle installed. No PATH changes, services, role enrollment or downloads. Hashes detect changes, not publisher authenticity."
	case "verify":
		if options.source != "" || options.stopped {
			return errors.New("verify accepts only --to")
		}
		if err := installation.Verify(options.target); err != nil {
			return err
		}
		result["notice"] = "Installed files match their local receipt; this is not a signature verification."
	case "remove":
		if !options.stopped || options.source != "" {
			return errors.New("remove requires --confirm-stopped and accepts no --from; this command does not stop services")
		}
		retained, err := installation.Remove(options.target)
		if err != nil {
			return err
		}
		result["retained"] = retained
		result["notice"] = "Installation moved aside, not deleted. Service state and agent configurations were not changed."
	default:
		return errors.New("bundle operation must be install, verify or remove")
	}
	if options.asJSON {
		return json.NewEncoder(out).Encode(result)
	}
	return writeBundleSummary(out, result)
}
