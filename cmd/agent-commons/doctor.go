// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"agentcommons/internal/core"
)

type healthReport struct {
	Ready    bool                    `json:"ready"`
	Identity string                  `json:"identity,omitempty"`
	Checks   []healthCheck           `json:"checks"`
	Notice   string                  `json:"notice"`
	Runtime  *core.RuntimeStatusPage `json:"runtime,omitempty"`
}

type healthCheck struct {
	Name string `json:"name"`
	OK   bool   `json:"ok"`
	Code string `json:"code,omitempty"`
}

func healthCheckFor(name string, err error) healthCheck {
	check := healthCheck{Name: name, OK: err == nil}
	if err != nil {
		check.Code = healthErrorCode(err)
	}
	return check
}

func healthErrorCode(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, os.ErrNotExist):
		return "not_found"
	case errors.Is(err, os.ErrPermission):
		return "permission"
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{"credential", "authenticated", "token", "authorization"} {
		if strings.Contains(message, marker) {
			return "authentication"
		}
	}
	if strings.Contains(message, "locked") || strings.Contains(message, "already in use") {
		return "busy"
	}
	return "unavailable"
}

func runDoctor(ctx context.Context, args []string, out, errOut io.Writer) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(errOut)
	config := fs.String("config", "", "private enrolled connection file; falls back to AGENT_COMMONS_CONNECTION or an upward .agent-commons/project.json search")
	timeout := fs.Duration("timeout", 5*time.Second, "total diagnostic deadline")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *timeout <= 0 || *timeout > time.Minute {
		return errors.New("doctor requires a timeout greater than zero, at most 1m, and no positional arguments")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	resolvedConfig, err := resolveConnectionConfigPath(*config, cwd)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	report := diagnoseConnection(ctx, resolvedConfig)
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
	report.Checks = append(report.Checks, healthCheckFor("private-connection-and-credential", err))
	if err != nil {
		return report
	}
	err = connection.verifyIdentity(ctx)
	report.Checks = append(report.Checks, healthCheckFor("authenticated-project-name-role", err))
	if err != nil {
		return report
	}
	report.Identity = connection.config.Identity
	for _, method := range []string{"inbox.page", "board.list"} {
		_, err = rpcCall[json.RawMessage](ctx, connection.client, method, map[string]int{"limit": 1})
		report.Checks = append(report.Checks, healthCheckFor(method, err))
		if err != nil {
			return report
		}
	}
	report.Ready = diagnoseRuntime(ctx, connection, &report)
	return report
}
