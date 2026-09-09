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

func runBundle(args []string, out, errOut io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: bundle install|verify|remove --to DIR [--from DIR] [--confirm-stopped]")
	}
	fs := flag.NewFlagSet("bundle "+args[0], flag.ContinueOnError)
	fs.SetOutput(errOut)
	source := fs.String("from", "", "extracted native archive directory")
	target := fs.String("to", "", "absolute dedicated installation directory")
	stopped := fs.Bool("confirm-stopped", false, "confirm all services using this installation have been stopped before removal")
	// One flag on the one FlagSet: --json means the same thing for install,
	// verify and remove, and is not something a caller has to look up per
	// subcommand.
	asJSON := fs.Bool("json", false, jsonFlagUsage)
	if err := parseFlags(fs, args[1:], out); err != nil {
		return err
	}
	if fs.NArg() != 0 || *target == "" {
		return errors.New("bundle requires --to and no positional arguments")
	}
	result := map[string]string{"operation": args[0], "directory": *target}
	switch args[0] {
	case "install":
		if *source == "" || *stopped {
			return errors.New("install requires --from; --confirm-stopped is only for remove")
		}
		if err := installation.Install(*source, *target); err != nil {
			return err
		}
		result["binary"] = filepath.Join(*target, "agent-commons")
		result["notice"] = "Local bundle installed. No PATH changes, services, role enrollment or downloads. Hashes detect changes, not publisher authenticity."
	case "verify":
		if *source != "" || *stopped {
			return errors.New("verify accepts only --to")
		}
		if err := installation.Verify(*target); err != nil {
			return err
		}
		result["notice"] = "Installed files match their local receipt; this is not a signature verification."
	case "remove":
		if !*stopped || *source != "" {
			return errors.New("remove requires --confirm-stopped and accepts no --from; this command does not stop services")
		}
		retained, err := installation.Remove(*target)
		if err != nil {
			return err
		}
		result["retained"] = retained
		result["notice"] = "Installation moved aside, not deleted. Service state and agent configurations were not changed."
	default:
		return errors.New("bundle operation must be install, verify or remove")
	}
	if *asJSON {
		return json.NewEncoder(out).Encode(result)
	}
	return writeBundleSummary(out, result)
}
