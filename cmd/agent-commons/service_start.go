// SPDX-License-Identifier: MPL-2.0

package main

import (
	"io"
	"os"
	"os/exec"
	"strings"
)

// Starting the local service as a detached process, and keeping hold of what it
// said if it never came up. Nothing here knows why the service was wanted.

// diagnosisLimit bounds what is read back out of the capture. The child decides
// how much it writes; init decides how much of that reaches an error message.
const diagnosisLimit = 4096

// serviceStartup is the handle on a spawned service's captured stderr. The
// process itself is deliberately not retained: it is released at start and
// outlives this one.
type serviceStartup struct{ stderrPath string }

// startDetached runs command as a detached process, capturing its stderr to a
// private file inside stateDir so a startup failure can be explained rather
// than merely reported.
//
// Both streams are real *os.File values, and that is load-bearing. Assigning an
// io.Writer -- io.Discard included -- makes os/exec build an OS pipe and park a
// copying goroutine in THIS process. Once init returns, that read end closes,
// and the detached service is killed by SIGPIPE the next time it writes to the
// descriptor. For stderr that is every successful RPC, because
// internal/transport logs one line per call: the service died on its first
// request and left the stale socket behind. A file has no reader to lose, no
// goroutine to leak, and nothing for the child to block on.
func startDetached(command *exec.Cmd, stateDir string) (*serviceStartup, error) {
	discard, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return nil, err
	}
	defer discard.Close()
	capture, err := os.CreateTemp(stateDir, "service-start-*.log")
	if err != nil {
		return nil, err
	}
	defer capture.Close()
	if err := capture.Chmod(0600); err != nil {
		return nil, err
	}
	startup := &serviceStartup{stderrPath: capture.Name()}
	// stdout stays discarded: the service has no stdout contract, and anything
	// it did write there would be noise in a diagnosis about not starting.
	command.Stdin, command.Stdout, command.Stderr = nil, discard, capture
	if err := command.Start(); err != nil {
		startup.discard()
		return nil, err
	}
	// The child holds its own duplicates of both descriptors from here; the
	// copies closed by the deferred Close above are this process's alone.
	_ = command.Process.Release()
	return startup, nil
}

// diagnose returns at most diagnosisLimit bytes of what the child printed. An
// unreadable or empty capture is reported as no diagnosis rather than as a
// second failure: the caller is already reporting one.
func (s *serviceStartup) diagnose() string {
	file, err := os.Open(s.stderrPath)
	if err != nil {
		return ""
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, diagnosisLimit))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(body))
}

// discard removes the capture file.
//
// A service that started successfully still holds its descriptor, so it goes on
// writing into an inode that no longer has a name and whose storage is
// reclaimed when it exits. That is what discarding the stream was always meant
// to mean, and what io.Discard did not do.
func (s *serviceStartup) discard() {
	if s == nil || s.stderrPath == "" {
		return
	}
	_ = os.Remove(s.stderrPath)
	s.stderrPath = ""
}
