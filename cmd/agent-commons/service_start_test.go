// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A service that dies during startup prints the reason and exits. init used to
// route that reason to io.Discard and then tell the operator to inspect a
// directory the reason was never written to. The capture must carry it back.
func TestDetachedStartCapturesWhyTheChildFailed(t *testing.T) {
	state := shortStateDir(t)
	startup, err := startDetached(exec.Command("/bin/sh", "-c",
		"echo 'state already locked: resource temporarily unavailable' >&2"), state)
	if err != nil {
		t.Fatal(err)
	}
	defer startup.discard()
	waitForDiagnosis(t, startup)
	if diagnosis := startup.diagnose(); !strings.Contains(diagnosis, "state already locked") {
		t.Fatalf("capture lost the child's reason: %q", diagnosis)
	}
}

// The child controls how much it writes; init does not. A failing service that
// prints without limit must not be able to grow init's error message without
// limit either.
//
// The ceiling here is a literal, deliberately NOT diagnosisLimit. Asserting
// against the constant would make the test move with it, and a bound that
// redefines itself bounds nothing -- raising the constant would keep the test
// green. Truncation is then confirmed against the capture on disk: the file
// holds far more than was read back.
func TestDetachedStartBoundsWhatItReadsBack(t *testing.T) {
	const ceiling = 8192
	state := shortStateDir(t)
	startup, err := startDetached(exec.Command("/bin/sh", "-c",
		"i=0; while [ $i -lt 4000 ]; do echo 'flooding the diagnosis buffer' >&2; i=$((i+1)); done"), state)
	if err != nil {
		t.Fatal(err)
	}
	defer startup.discard()
	written := waitForCapture(t, startup, 4*ceiling)
	if got := len(startup.diagnose()); got > ceiling {
		t.Fatalf("diagnosis read back %d bytes, more than the %d byte ceiling", got, ceiling)
	}
	if got := len(startup.diagnose()); int64(got) >= written {
		t.Fatalf("diagnosis read back %d bytes of a %d byte capture; nothing was truncated", got, written)
	}
}

// The capture is a real file, never an io.Writer. An io.Writer makes os/exec
// build a pipe and park a copying goroutine in this process; when the parent
// exits, the read end closes and the detached service is killed by SIGPIPE on
// its next write. Neither stream may be a pipe.
func TestDetachedStartGivesTheChildNoPipes(t *testing.T) {
	state := shortStateDir(t)
	command := exec.Command("/bin/sh", "-c", "exit 0")
	startup, err := startDetached(command, state)
	if err != nil {
		t.Fatal(err)
	}
	defer startup.discard()
	for name, stream := range map[string]any{"stdout": command.Stdout, "stderr": command.Stderr} {
		if _, ok := stream.(*os.File); !ok {
			t.Fatalf("%s is %T, not an *os.File; os/exec will interpose a pipe", name, stream)
		}
	}
	if len(command.ExtraFiles) != 0 {
		t.Fatalf("detached child inherited %d extra descriptors", len(command.ExtraFiles))
	}
}

// discard removes the capture so a successful start leaves nothing behind in
// the private state directory.
func TestDetachedStartDiscardsTheCaptureOnSuccess(t *testing.T) {
	state := shortStateDir(t)
	startup, err := startDetached(exec.Command("/bin/sh", "-c", "exit 0"), state)
	if err != nil {
		t.Fatal(err)
	}
	path := startup.stderrPath
	startup.discard()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("capture %s survived discard: %v", path, err)
	}
}

// waitForDiagnosis polls until the child's output lands. The child is detached
// and deliberately not waited on, so its writes are not ordered against this
// test's reads.
func waitForDiagnosis(t *testing.T, startup *serviceStartup) {
	t.Helper()
	for attempt := 0; attempt < 200; attempt++ {
		if startup.diagnose() != "" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("child produced no output")
}

// waitForCapture polls until the capture holds at least atLeast bytes and
// returns its size, so a bound can be checked against what the child actually
// produced rather than against the constant that bounds it.
func waitForCapture(t *testing.T, startup *serviceStartup, atLeast int64) int64 {
	t.Helper()
	for attempt := 0; attempt < 400; attempt++ {
		if info, err := os.Stat(startup.stderrPath); err == nil && info.Size() >= atLeast {
			return info.Size()
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("child never wrote %d bytes", atLeast)
	return 0
}

// shortStateDir keeps the state path well inside the ~104 byte limit a unix
// socket path has, which t.TempDir() alone does not guarantee.
func shortStateDir(t *testing.T) string {
	t.Helper()
	state, err := os.MkdirTemp("", "acs")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(state) })
	if err := os.Chmod(state, 0700); err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(state)
}

// init removes the capture on every path out of runInit, not only on the paths
// service_start's own tests cover. Without that wiring a real service start
// leaves a service-start-*.log in the private state directory permanently, and
// the design's stated invariant quietly stops holding.
//
// The spawn is injected: from a test binary os.Executable() is the test binary,
// so the real spawn would run this whole suite as a child.
func TestInitRemovesTheStartupCaptureOnEveryPath(t *testing.T) {
	state, target := shortStateDir(t), shortStateDir(t)
	var out, errOut bytes.Buffer
	err := runInitWith(context.Background(), []string{"--state", state, "--target", target,
		"--name", "lead", "--role", "workspace-lead", "--runtime", "codex"}, &out, &errOut,
		func(string, string) (*serviceStartup, error) {
			return startDetached(exec.Command("/bin/sh", "-c",
				"echo 'state already locked: resource temporarily unavailable' >&2"), state)
		})
	if err == nil {
		t.Fatal("init reported success though no service was started")
	}
	// The captured reason reaches the operator rather than a directory to search.
	if !strings.Contains(err.Error(), "state already locked") {
		t.Fatalf("init lost the child's reason: %v", err)
	}
	leftovers, globErr := filepath.Glob(filepath.Join(state, "service-start-*.log"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(leftovers) != 0 {
		t.Fatalf("init left %d startup capture(s) behind: %v", len(leftovers), leftovers)
	}
}
