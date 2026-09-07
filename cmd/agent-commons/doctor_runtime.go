// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"

	"agentcommons/internal/core"
)

func diagnoseRuntime(ctx context.Context, connection projectConnection, report *healthReport) bool {
	capabilities, err := rpcCall[struct {
		RuntimeStatusAvailable bool `json:"runtimeStatusAvailable"`
	}](ctx, connection.client, "sessions.capabilities", struct{}{})
	return runtimeDoctorChecks(report, capabilities.RuntimeStatusAvailable, err, func() (core.RuntimeStatusPage, error) {
		return rpcCall[core.RuntimeStatusPage](ctx, connection.client, "runtime.status", struct{}{})
	})
}

func runtimeDoctorChecks(report *healthReport, available bool, capabilityErr error, readStatus func() (core.RuntimeStatusPage, error)) bool {
	report.Checks = append(report.Checks, healthCheckFor("sessions.capabilities", capabilityErr))
	if capabilityErr != nil {
		return false
	}
	if !available {
		return true
	}
	status, err := readStatus()
	report.Checks = append(report.Checks, healthCheckFor("runtime.status", err))
	if err != nil {
		return false
	}
	report.Runtime = &status
	return true
}
