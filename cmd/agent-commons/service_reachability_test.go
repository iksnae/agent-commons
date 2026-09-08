// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// refusedError and missingError are the two dial failures the transport really
// produces. They are built by dialling for real rather than by hand, so the
// classifier is tested against the error shape it will actually be handed.
func refusedError(t *testing.T) error {
	t.Helper()
	socket := filepath.Join(shortStateDir(t), "stale.sock")
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socket, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	listener.SetUnlinkOnClose(false)
	listener.Close()
	_, err = net.Dial("unix", socket)
	if err == nil {
		t.Fatal("dial to a listener-less socket succeeded")
	}
	if !errors.Is(err, syscall.ECONNREFUSED) {
		t.Fatalf("fixture produced %v, not ECONNREFUSED", err)
	}
	return err
}

func missingError(t *testing.T) error {
	t.Helper()
	_, err := net.Dial("unix", filepath.Join(shortStateDir(t), "absent.sock"))
	if err == nil {
		t.Fatal("dial to a missing socket succeeded")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("fixture produced %v, not ErrNotExist", err)
	}
	return err
}

// populatedState is a state directory a service has run in: it has the operator
// credential that absence is judged by.
func populatedState(t *testing.T) string {
	t.Helper()
	state := shortStateDir(t)
	if err := os.WriteFile(filepath.Join(state, "operator.token"), []byte("fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return state
}

// The three conditions are told apart by evidence, not by matching strings in a
// message. Each row names the evidence that distinguishes it.
func TestConditionsAreClassifiedByEvidence(t *testing.T) {
	populated := populatedState(t)
	empty := shortStateDir(t)
	for _, row := range []struct {
		name  string
		state string
		err   error
		want  serviceCondition
	}{
		{"never run here, socket missing", empty, missingError(t), serviceAbsent},
		{"never run here, socket left behind", empty, refusedError(t), serviceAbsent},
		{"clean shutdown removed the socket", populated, missingError(t), serviceStopped},
		{"died hard and left the socket", populated, refusedError(t), serviceStale},
	} {
		if got := classifyServiceCondition(row.state, row.err); got != row.want {
			t.Fatalf("%s: classified %v, want %v", row.name, got, row.want)
		}
	}
}

// An error that is not a failure to reach the service must not be dressed up as
// one. A rejected credential is still a rejected credential.
func TestUnrelatedErrorsAreNotClassified(t *testing.T) {
	state := populatedState(t)
	for _, err := range []error{
		errors.New("unauthorized"),
		errors.New("unknown method"),
		context.DeadlineExceeded,
	} {
		if got := classifyServiceCondition(state, err); got != serviceConditionUnknown {
			t.Fatalf("%v classified as %v", err, got)
		}
	}
}

// --socket lets an operator point at a socket with no matching state directory.
// That must degrade rather than error: stale and stopped stay separable, and
// absent -- which only the state directory can evidence -- is simply not
// claimed.
func TestNoStateDirectoryDegradesRatherThanErroring(t *testing.T) {
	if got := classifyServiceCondition("", refusedError(t)); got != serviceStale {
		t.Fatalf("stale not separable without a state directory: %v", got)
	}
	if got := classifyServiceCondition("", missingError(t)); got != serviceStopped {
		t.Fatalf("stopped not separable without a state directory: %v", got)
	}
}

// The staleness test is recoverServiceSocket's, reused. If these ever disagree,
// the CLI is calling a socket stale that serve will refuse to replace.
func TestStalenessMatchesSocketRecovery(t *testing.T) {
	if !socketRefused(refusedError(t)) {
		t.Fatal("socketRefused rejects the error recovery treats as stale")
	}
	if socketRefused(missingError(t)) {
		t.Fatal("socketRefused accepts a missing socket as stale")
	}
}

// Every condition renders a sentence. A condition added without one would show
// the operator a blank line where the diagnosis should be.
func TestEveryConditionNamesItselfAndAFix(t *testing.T) {
	headlines := map[string]bool{}
	for condition := serviceCondition(0); condition < serviceConditionCount; condition++ {
		sentence := serviceSentences[condition]
		if strings.TrimSpace(sentence.headline) == "" {
			t.Fatalf("condition %d has no headline", condition)
		}
		if strings.TrimSpace(sentence.fix) == "" {
			t.Fatalf("condition %d has no fix", condition)
		}
		if headlines[sentence.headline] {
			t.Fatalf("condition %d repeats a headline: %q", condition, sentence.headline)
		}
		headlines[sentence.headline] = true
	}
}

// Machine-facing commands get one line and nothing on stdout. The line still
// has to say which condition it is and what to do.
func TestUnreachableErrorIsOneInformativeLine(t *testing.T) {
	for condition, want := range map[serviceCondition]string{
		serviceAbsent:  "service_absent",
		serviceStopped: "service_stopped",
		serviceStale:   "service_stale",
	} {
		err := &serviceUnreachableError{condition: condition, socket: "/s/service.sock", err: errors.New("dial")}
		line := err.Error()
		if strings.Contains(line, "\n") {
			t.Fatalf("%s spans more than one line: %q", want, line)
		}
		if !strings.Contains(line, serviceSentences[condition].headline) {
			t.Fatalf("%s omits its headline: %q", want, line)
		}
		if !strings.Contains(line, serviceSentences[condition].fix) {
			t.Fatalf("%s omits its fix: %q", want, line)
		}
	}
}

// The original error is wrapped, not replaced: the sentinel a caller classified
// on is still reachable through the typed error.
func TestUnreachableErrorWrapsItsCause(t *testing.T) {
	cause := refusedError(t)
	err := error(&serviceUnreachableError{condition: serviceStale, err: cause})
	if !errors.Is(err, syscall.ECONNREFUSED) {
		t.Fatal("wrapping lost the transport sentinel")
	}
	var unreachable *serviceUnreachableError
	if !errors.As(err, &unreachable) || unreachable.condition != serviceStale {
		t.Fatal("typed error is not recoverable with errors.As")
	}
}

// rpcCall is the seam. A dial that fails must come back typed, so one renderer
// can recognise it however far up it is returned.
func TestRPCCallReturnsTheTypedError(t *testing.T) {
	state := populatedState(t)
	client := rpcClient{socket: filepath.Join(state, "service.sock"), token: "fixture", state: state}
	_, err := rpcCall[map[string]any](context.Background(), client, "sessions.list", struct{}{})
	var unreachable *serviceUnreachableError
	if !errors.As(err, &unreachable) {
		t.Fatalf("rpcCall returned %T (%v), not a classified unreachable error", err, err)
	}
	if unreachable.condition != serviceStopped {
		t.Fatalf("classified as %v, want stopped", unreachable.condition)
	}
}

// doctor turns errors into a report rather than returning them, so it needs the
// classification ahead of its string sniffing. These codes are additive values
// on an existing machine contract.
func TestDoctorCodesNameTheServiceCondition(t *testing.T) {
	for condition, want := range map[serviceCondition]string{
		serviceAbsent:  "service_absent",
		serviceStopped: "service_stopped",
		serviceStale:   "service_stale",
	} {
		err := error(&serviceUnreachableError{condition: condition, err: errors.New("dial")})
		if got := healthErrorCode(err); got != want {
			t.Fatalf("condition %v produced code %q, want %q", condition, got, want)
		}
	}
}

// The classification must beat the sniffing it was put in front of. A stale
// socket's cause reads "connection refused", which the old rules would have
// filed as the catch-all.
func TestServiceConditionOutranksStringSniffing(t *testing.T) {
	err := error(&serviceUnreachableError{condition: serviceStale, err: refusedError(t)})
	if got := healthErrorCode(err); got != "service_stale" {
		t.Fatalf("string sniffing won: %q", got)
	}
}

// Human-facing commands get the styled block on stderr. run owns that
// rendering, so no command has to remember to do it, and a test driving run
// with buffers still sees it.
//
// enroll is the command used here because it dials without auto-starting
// anything: the state directory below has a credential and no service, which is
// the stopped condition exactly.
func TestRunRendersTheBlockOnStderrAndLeavesStdoutAlone(t *testing.T) {
	state := populatedState(t)
	target := shortStateDir(t)
	var out, errOut strings.Builder
	err := run(context.Background(), []string{"enroll", "--state", state, "--target", target,
		"--name", "lead", "--role", "workspace-lead"}, strings.NewReader(""), &out, &errOut)
	var unreachable *serviceUnreachableError
	if !errors.As(err, &unreachable) {
		t.Fatalf("enroll returned %T (%v), not a classified unreachable error", err, err)
	}
	if unreachable.condition != serviceStopped {
		t.Fatalf("classified as %v, want stopped", unreachable.condition)
	}
	block := errOut.String()
	if !strings.Contains(block, serviceSentences[serviceStopped].headline) {
		t.Fatalf("stderr lacks the condition: %q", block)
	}
	if !strings.Contains(block, serviceSentences[serviceStopped].fix) {
		t.Fatalf("stderr lacks the fix: %q", block)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout was written to: %q", out.String())
	}
}
