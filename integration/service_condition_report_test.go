// SPDX-License-Identifier: MPL-2.0

package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The unreachable-service diagnosis is rendered by run and printed by main's
// process boundary, and no test that drives run can see both. It printed twice
// for every human-facing command on exactly that gap: the styled block, then
// the same headline, fix and socket path again as one line underneath.
//
// This builds the real binary and reads its stderr, so the double print cannot
// come back unnoticed. It also fixes the division of labour between the two
// audiences: a person gets the block, a parser gets the line, and neither gets
// both.
func TestStoppedServiceIsReportedOnceByTheBuiltBinary(t *testing.T) {
	binary := buildCommonsBinary(t)

	// A populated state directory with no socket is the stopped condition:
	// serve removes its socket on a clean shutdown.
	state, err := os.MkdirTemp("/tmp", "accond")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(state) })
	if err := os.Chmod(state, 0700); err != nil {
		t.Fatal(err)
	}
	token := filepath.Join(state, "operator.token")
	if err := os.WriteFile(token, []byte("fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// Sentences the two forms share. If either form is dropped or repeated,
	// one of these counts moves off 1.
	const (
		headline = "The local service is not running. It shut down cleanly."
		fix      = "Start it with 'agent-commons service start'"
	)
	socket := filepath.Join(state, "service.sock")

	t.Run("human-facing watch gets the block once", func(t *testing.T) {
		stdout, stderr, code := runCommons(t, binary, "watch", "--state", state,
			"--token-file", token, "--interval", "100ms", "--timeout", "3s")
		if code == 0 {
			t.Fatalf("watch succeeded against a stopped service:\n%s", stderr)
		}
		for _, phrase := range []string{headline, fix, socket} {
			if got := strings.Count(stderr, phrase); got != 1 {
				t.Fatalf("stderr repeats %q %d times, want exactly 1:\n%s", phrase, got, stderr)
			}
		}
		// Counting phrases proves it is not repeated; it does not prove the
		// human form is the block. Only the block puts the paths on their own
		// labelled rows, so this is what separates it from the single line.
		if !strings.Contains(stderr, "Service state") {
			t.Fatalf("human-facing stderr is not the styled block:\n%s", stderr)
		}
		if lines := strings.Count(strings.TrimSpace(stderr), "\n"); lines < 2 {
			t.Fatalf("human-facing stderr is a single line, not a block:\n%s", stderr)
		}
		if stdout != "" {
			t.Fatalf("watch wrote to stdout: %q", stdout)
		}
	})

	t.Run("machine-facing call gets one line and no block", func(t *testing.T) {
		stdout, stderr, code := runCommons(t, binary, "call", "--state", state,
			"--token-file", token, "sessions.list")
		if code == 0 {
			t.Fatalf("call succeeded against a stopped service:\n%s", stderr)
		}
		for _, phrase := range []string{headline, fix, socket} {
			if got := strings.Count(stderr, phrase); got != 1 {
				t.Fatalf("stderr repeats %q %d times, want exactly 1:\n%s", phrase, got, stderr)
			}
		}
		// One line, so no styled block: the block puts the socket on its own
		// labelled row, which the single line never does.
		if lines := strings.Count(strings.TrimSpace(stderr), "\n"); lines != 0 {
			t.Fatalf("machine-facing stderr is not a single line:\n%s", stderr)
		}
		if stdout != "" {
			t.Fatalf("call wrote to stdout: %q", stdout)
		}
	})

	// --json selects the report form on stdout; it does not reach this block,
	// which run chooses from args[0] alone after the command's FlagSet is gone
	// (see jsonFlagUsage in cmd/agent-commons/presentation.go). So these four
	// ask for machine output and still get the styled block on stderr.
	//
	// That is tolerable for exactly one reason, and this is the reason: stdout
	// stays byte-empty, so a caller parsing it reads nothing rather than
	// reading prose. The comment says the cost is confined to stderr; without
	// this, nothing holds it there. If a future change ever renders the block
	// on stdout for one of these, the claim silently becomes false and a
	// parser starts consuming box-aligned text.
	target, err := os.MkdirTemp("/tmp", "actgt")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(target) })

	// A connection file pointing at the same absent socket, so the commands
	// that read one reach the dial rather than stopping at resolution.
	config := filepath.Join(state, "connection.json")
	if err := os.WriteFile(config, []byte(fmt.Sprintf(
		`{"version":1,"identity":"sess-fixture","target":%q,"name":"n","role":"r","socket":%q,"tokenFile":%q,"state":%q}`,
		target, socket, token, state)), 0600); err != nil {
		t.Fatal(err)
	}

	for _, machine := range []struct {
		name string
		argv []string
	}{
		{"enroll --json", []string{"enroll", "--state", state, "--name", "n", "--role", "r", "--target", target, "--json"}},
		{"retire --json", []string{"retire", "--state", state, "--id", "sess-fixture", "--evidence", "e", "--json"}},
		{"reinstate --json", []string{"reinstate", "--state", state, "--id", "sess-fixture", "--evidence", "e", "--json"}},
		{"console --once", []string{"console", "--config", config, "--once"}},
	} {
		t.Run(machine.name+" leaves stdout empty", func(t *testing.T) {
			stdout, stderr, code := runCommons(t, binary, machine.argv...)
			if code == 0 {
				t.Fatalf("%s succeeded against a stopped service:\n%s", machine.name, stderr)
			}
			if stdout != "" {
				t.Fatalf("%s wrote to stdout on the unreachable path: %q", machine.name, stdout)
			}
			// Naming the block is what makes this row evidence for the comment
			// rather than a bare emptiness check: it records that the block IS
			// what these receive, which is the inconsistency being tolerated.
			if !strings.Contains(stderr, "Service state") {
				t.Fatalf("%s did not get the styled block; jsonFlagUsage in presentation.go says it does:\n%s", machine.name, stderr)
			}
		})
	}
}

func buildCommonsBinary(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "agent-commons")
	build := exec.Command("go", "build", "-o", binary, "./cmd/agent-commons")
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v: %s", err, output)
	}
	return binary
}

func runCommons(t *testing.T, binary string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var outBuf, errBuf strings.Builder
	command := exec.Command(binary, args...)
	command.Stdout, command.Stderr = &outBuf, &errBuf
	err := command.Run()
	var exit *exec.ExitError
	if err != nil && !asExitError(err, &exit) {
		t.Fatalf("%v: %v", args, err)
	}
	return outBuf.String(), errBuf.String(), command.ProcessState.ExitCode()
}

func asExitError(err error, target **exec.ExitError) bool {
	exit, ok := err.(*exec.ExitError)
	if ok {
		*target = exit
	}
	return ok
}
