// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"slices"
	"strings"

	"agentcommons/internal/supervision"
)

// serviceOperations is the list the operation check reads and the usage
// sentence and help screen are written from, so the screen cannot name an
// operation the check rejects, and cannot omit one it accepts.
var serviceOperations = []string{"install", "enable", "start", "status", "stop", "remove"}

// serviceFlags is what service parses, registered in one function so the
// listing `service --help` prints is PrintDefaults over the same FlagSet a
// subcommand form parses rather than a transcription of it.
type serviceFlags struct {
	directory string
	file      string
	confirm   bool
	options   supervision.Options
}

func serviceFlagSet(name string, errOut io.Writer) (*flag.FlagSet, *serviceFlags) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(errOut)
	parsed := &serviceFlags{options: supervision.Options{Platform: runtime.GOOS}}
	fs.StringVar(&parsed.directory, "directory", "", "existing service configuration directory (install only)")
	fs.StringVar(&parsed.options.Binary, "binary", "", "absolute executable path (install only)")
	fs.StringVar(&parsed.options.State, "state", "", "private state path (install only)")
	fs.StringVar(&parsed.options.SearchPath, "path", "", "explicit provider CLI PATH (install only)")
	fs.StringVar(&parsed.file, "file", "", "installed service configuration file")
	fs.BoolVar(&parsed.confirm, "confirm-stopped", false, "confirm service is stopped before removal")
	return fs, parsed
}

func runService(ctx context.Context, args []string, out, errOut io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: service %s [options]", strings.Join(serviceOperations, "|"))
	}
	// Asked before args[0] is checked as an operation name. Without this the
	// operator asking for help is told --help is not one, which is true and
	// useless.
	if helpRequested(args) {
		fs, _ := serviceFlagSet("service", errOut)
		return writeCommandHelp(out, fs, "agent-commons service "+strings.Join(serviceOperations, "|")+" [flags]", []string{
			"",
			"install   write a supervisor configuration file for this host",
			"enable    register the installed configuration with the supervisor",
			"start     ask the supervisor to start the service now",
			"status    report what the supervisor knows about the service",
			"stop      ask the supervisor to stop the service",
			"remove    disable the service and move its configuration aside",
			"",
			"A supervisor command completing does not prove the service is ready.",
			"Run 'agent-commons doctor' for scoped connection checks.",
		})
	}
	operation := args[0]
	if !slices.Contains(serviceOperations, operation) {
		return fmt.Errorf("unknown service operation: %s", operation)
	}
	fs, parsed := serviceFlagSet("service "+operation, errOut)
	if err := parseFlags(fs, args[1:], out); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected service arguments")
	}
	if operation == "install" {
		if parsed.file != "" || parsed.confirm {
			return fmt.Errorf("install does not accept --file or --confirm-stopped")
		}
		installed, err := supervision.InstallFile(parsed.directory, parsed.options)
		if err != nil {
			return err
		}
		return json.NewEncoder(out).Encode(map[string]string{"operation": operation, "file": installed, "notice": "Configuration installed; no supervisor command run. A plist in LaunchAgents may load at next login. Enable and start explicitly for this session. State and agents unchanged."})
	}
	if parsed.directory != "" || parsed.options.Binary != "" || parsed.options.State != "" || parsed.options.SearchPath != "" {
		return fmt.Errorf("installation options are only valid for install")
	}
	if operation != "remove" && parsed.confirm {
		return fmt.Errorf("--confirm-stopped is only valid for removal")
	}
	if operation == "remove" && !parsed.confirm {
		return fmt.Errorf("stop the service first, then pass --confirm-stopped; removal does not inspect processes")
	}
	return controlService(ctx, operation, parsed.file, out, supervision.NativeCommands{})
}

func controlService(ctx context.Context, operation, file string, out io.Writer, commands supervision.Commander) error {
	installed, err := supervision.ReadInstalled(file)
	if err != nil {
		return err
	}
	if installed.Options.Platform != runtime.GOOS {
		return fmt.Errorf("installed service platform differs from this host")
	}
	control := supervision.Control{Platform: runtime.GOOS, UID: os.Getuid(), File: file, Label: installed.Plan.Label, Commands: commands}
	action := operation
	if action == "remove" {
		action = "disable"
	}
	result, err := control.Apply(ctx, action)
	if err != nil {
		return err
	}
	response := map[string]string{"operation": operation, "file": file, "output": result, "notice": "Supervisor command completed; this does not prove Agent Commons RPC or providers are ready. Use doctor for scoped connection checks."}
	if operation == "remove" {
		retained, err := supervision.RetainFiles(file)
		if err != nil {
			return err
		}
		response["retained"] = retained
		if runtime.GOOS == "linux" {
			if _, err := control.Apply(ctx, "reload"); err != nil {
				return fmt.Errorf("configuration retained at %s; post-removal reload failed: %w", retained, err)
			}
		}
		response["notice"] = "Service disabled and configuration moved aside, not deleted. Agent state retained. Process shutdown relied on your --confirm-stopped confirmation."
	}
	return json.NewEncoder(out).Encode(response)
}
