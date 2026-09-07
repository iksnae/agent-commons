// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"
)

type healthReport struct {
	Ready    bool          `json:"ready"`
	Identity string        `json:"identity,omitempty"`
	Checks   []healthCheck `json:"checks"`
	Notice   string        `json:"notice"`
}

type healthCheck struct {
	Name string `json:"name"`
	OK   bool   `json:"ok"`
}

func runDoctor(ctx context.Context, args []string, out, errOut io.Writer) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(errOut)
	config := fs.String("config", "", "private enrolled connection file (required)")
	timeout := fs.Duration("timeout", 5*time.Second, "total diagnostic deadline")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *config == "" || fs.NArg() != 0 || *timeout <= 0 || *timeout > time.Minute {
		return errors.New("doctor requires --config and a timeout greater than zero, at most 1m")
	}
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	report := diagnoseConnection(ctx, *config)
	if err := json.NewEncoder(out).Encode(report); err != nil {
		return err
	}
	if !report.Ready {
		return fmt.Errorf("connection check failed; inspect the named check (credentials and peer data are omitted)")
	}
	return nil
}

func diagnoseConnection(ctx context.Context, path string) healthReport {
	report := healthReport{Notice: "Read-only connection check. No attachment, acknowledgement, enrollment or model wake. Ready means scoped RPC checks passed, not production readiness or provider availability."}
	connection, err := openProjectConnection(path)
	report.Checks = append(report.Checks, healthCheck{"private-connection-and-credential", err == nil})
	if err != nil {
		return report
	}
	err = connection.verifyIdentity(ctx)
	report.Checks = append(report.Checks, healthCheck{"authenticated-project-name-role", err == nil})
	if err != nil {
		return report
	}
	report.Identity = connection.config.Identity
	for _, method := range []string{"inbox.page", "board.list"} {
		_, err = rpcCall[json.RawMessage](ctx, connection.client, method, map[string]int{"limit": 1})
		report.Checks = append(report.Checks, healthCheck{method, err == nil})
		if err != nil {
			return report
		}
	}
	report.Ready = true
	return report
}
