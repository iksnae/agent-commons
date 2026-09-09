// SPDX-License-Identifier: MPL-2.0

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"runtime"

	"agentcommons/internal/supervision"
)

func runServicePlan(args []string, out, errOut io.Writer) error {
	fs := flag.NewFlagSet("service-plan", flag.ContinueOnError)
	fs.SetOutput(errOut)
	var options supervision.Options
	fs.StringVar(&options.Platform, "platform", runtime.GOOS, "darwin or linux")
	fs.StringVar(&options.Binary, "binary", "", "absolute installed executable path")
	fs.StringVar(&options.State, "state", "", "absolute private service state directory")
	fs.StringVar(&options.SearchPath, "path", "", "explicit PATH for authenticated Claude/Codex CLIs")
	if err := parseFlags(fs, args, out); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected service-plan arguments")
	}
	plan, err := supervision.Render(options)
	if err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(plan)
}
