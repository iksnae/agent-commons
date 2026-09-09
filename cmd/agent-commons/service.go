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

	"agentcommons/internal/supervision"
)

func runService(ctx context.Context, args []string, out, errOut io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: service install|enable|start|status|stop|remove [options]")
	}
	operation := args[0]
	switch operation {
	case "install", "enable", "start", "status", "stop", "remove":
	default:
		return fmt.Errorf("unknown service operation: %s", operation)
	}
	fs := flag.NewFlagSet("service "+operation, flag.ContinueOnError)
	fs.SetOutput(errOut)
	var directory, file string
	var confirm bool
	options := supervision.Options{Platform: runtime.GOOS}
	fs.StringVar(&directory, "directory", "", "existing service configuration directory (install only)")
	fs.StringVar(&options.Binary, "binary", "", "absolute executable path (install only)")
	fs.StringVar(&options.State, "state", "", "private state path (install only)")
	fs.StringVar(&options.SearchPath, "path", "", "explicit provider CLI PATH (install only)")
	fs.StringVar(&file, "file", "", "installed service configuration file")
	fs.BoolVar(&confirm, "confirm-stopped", false, "confirm service is stopped before removal")
	if err := parseFlags(fs, args[1:], out); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected service arguments")
	}
	if operation == "install" {
		if file != "" || confirm {
			return fmt.Errorf("install does not accept --file or --confirm-stopped")
		}
		installed, err := supervision.InstallFile(directory, options)
		if err != nil {
			return err
		}
		return json.NewEncoder(out).Encode(map[string]string{"operation": operation, "file": installed, "notice": "Configuration installed; no supervisor command run. A plist in LaunchAgents may load at next login. Enable and start explicitly for this session. State and agents unchanged."})
	}
	if directory != "" || options.Binary != "" || options.State != "" || options.SearchPath != "" {
		return fmt.Errorf("installation options are only valid for install")
	}
	if operation != "remove" && confirm {
		return fmt.Errorf("--confirm-stopped is only valid for removal")
	}
	if operation == "remove" && !confirm {
		return fmt.Errorf("stop the service first, then pass --confirm-stopped; removal does not inspect processes")
	}
	return controlService(ctx, operation, file, out, supervision.NativeCommands{})
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
