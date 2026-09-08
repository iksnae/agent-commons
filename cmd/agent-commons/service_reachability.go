// SPDX-License-Identifier: MPL-2.0

package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// Telling apart the three ways the local service can be out of reach. This file
// classifies; it says nothing to anybody. The sentences for each condition live
// in human_reports.go, and the decision to show them lives in main.go.

// serviceCondition is why a dial did not reach a service.
type serviceCondition int

const (
	// serviceConditionUnknown means the failure was not a failure to reach the
	// service at all -- a rejected credential, an unknown method, a deadline.
	// Nothing is claimed about the service, and nothing is rewritten.
	serviceConditionUnknown serviceCondition = iota
	// serviceAbsent: no state directory or no operator credential in it. The
	// service has never run here.
	serviceAbsent
	// serviceStopped: the state directory is populated and the socket path is
	// gone. serve removes its socket on SIGTERM, so this is a clean shutdown.
	serviceStopped
	// serviceStale: the socket file is still there and refuses connections. The
	// service died without removing it.
	serviceStale
	// serviceConditionCount bounds the conditions. serviceSentences is an array
	// of this length, so a condition added above widens that array and the
	// entry it gains is empty until somebody writes it --
	// TestEveryConditionNamesItselfAndAFix is what notices.
	serviceConditionCount
)

// socketRefused is the single staleness test in this program. recoverServiceSocket
// uses it to decide a default socket may be replaced, and classification uses it
// to decide a socket is stale; if the two ever disagreed, the CLI would call a
// socket stale that serve then refuses to replace.
func socketRefused(err error) bool { return errors.Is(err, syscall.ECONNREFUSED) }

// classifyServiceCondition names why a dial failed, from evidence rather than
// from the text of the message.
//
// state may be empty. The --socket override lets an operator point at a socket
// with no matching state directory, and that degrades rather than errors: stale
// and stopped are still separable, because the dial itself distinguishes them.
// Absent is the one condition only the state directory can evidence, so without
// one it is simply not claimed.
func classifyServiceCondition(state string, err error) serviceCondition {
	refused, missing := socketRefused(err), errors.Is(err, fs.ErrNotExist)
	// Anything that is not one of the two dial failures is not a reachability
	// problem. Checking this first also keeps a missing credential from
	// relabelling an unrelated error as absence.
	if !refused && !missing {
		return serviceConditionUnknown
	}
	if state != "" {
		if _, err := os.Stat(filepath.Join(state, "operator.token")); errors.Is(err, fs.ErrNotExist) {
			return serviceAbsent
		}
	}
	if refused {
		return serviceStale
	}
	return serviceStopped
}

// code is the stable token doctor reports for a condition. These are values on
// an existing machine contract, not prose.
func (c serviceCondition) code() string {
	switch c {
	case serviceAbsent:
		return "service_absent"
	case serviceStopped:
		return "service_stopped"
	case serviceStale:
		return "service_stale"
	default:
		return "unavailable"
	}
}

// serviceUnreachableError is a classified dial failure on its way up to
// whoever renders it. It carries no sentences: Error() borrows the machine-facing
// one from human_reports.go, which owns every sentence an operator reads.
type serviceUnreachableError struct {
	condition serviceCondition
	socket    string
	state     string
	err       error
}

func (e *serviceUnreachableError) Error() string { return serviceUnreachableLine(e) }

// Unwrap keeps the transport sentinel reachable, so a caller that classified on
// errors.Is can still do so after this wrapping.
func (e *serviceUnreachableError) Unwrap() error { return e.err }
