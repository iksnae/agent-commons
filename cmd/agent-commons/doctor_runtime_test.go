// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"errors"
	"os"
	"testing"

	"agentcommons/internal/core"
)

func TestDoctorRuntimeNegotiationDoesNotInventAvailability(t *testing.T) {
	for _, test := range []struct {
		name                     string
		available                bool
		capabilityErr, statusErr error
		wantReady, wantCall      bool
	}{
		{name: "legacy", wantReady: true},
		{name: "capability-failed", capabilityErr: errors.New("unavailable")},
		{name: "advertised-but-failed", available: true, statusErr: errors.New("unavailable"), wantCall: true},
		{name: "supported", available: true, wantReady: true, wantCall: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			report := healthReport{}
			called := false
			ready := runtimeDoctorChecks(&report, test.available, test.capabilityErr, func() (core.RuntimeStatusPage, error) {
				called = true
				return core.RuntimeStatusPage{}, test.statusErr
			})
			if ready != test.wantReady || called != test.wantCall || (report.Runtime != nil) != (test.wantCall && test.statusErr == nil) {
				t.Fatal("runtime diagnostic negotiation violated availability boundary")
			}
			if test.statusErr != nil && (len(report.Checks) != 2 || report.Checks[1].OK) {
				t.Fatal("advertised status failure not reported")
			}
		})
	}
}

func TestHealthErrorCodesStayWithinSafeContract(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want string
	}{
		{"deadline", context.DeadlineExceeded, "deadline"},
		{"canceled", context.Canceled, "canceled"},
		{"not-found", os.ErrNotExist, "not_found"},
		{"permission", os.ErrPermission, "permission"},
		{"authentication", errors.New("configured credential rejected"), "authentication"},
		{"busy", errors.New("state already locked"), "busy"},
		{"unavailable", errors.New("socket closed"), "unavailable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := healthErrorCode(test.err); got != test.want {
				t.Fatalf("code = %q, want %q", got, test.want)
			}
		})
	}
}
