// SPDX-License-Identifier: MPL-2.0

package codexlaunch

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestBindingLeaseReleasesAfterOwnerProcessDies(t *testing.T) {
	path, scope := readyJournalFixture(t)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "-test.run", "^TestBindingLeaseProcessFixture$", "--", "--lease", path)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	if line, err := bufio.NewReader(stdout).ReadString('\n'); err != nil || line != "locked\n" {
		t.Fatal("child did not acquire binding", err)
	}
	if second, err := AcquireReady(path, scope); !errors.Is(err, ErrBindingBusy) {
		if second != nil {
			_ = second.Close()
		}
		t.Fatal("second process acquired held binding", err)
	}
	if err = cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err = cmd.Wait(); err == nil || cmd.ProcessState == nil {
		t.Fatal("fixture process did not terminate", err)
	}
	lease, err := AcquireReady(path, scope)
	if err != nil {
		t.Fatal("dead process retained lock", err)
	}
	defer lease.Close()
}

func TestBindingLeaseProcessFixture(t *testing.T) {
	args := os.Args
	if len(args) < 3 || args[len(args)-2] != "--lease" {
		return
	}
	scope := Scope{Identity: "lead", Target: "/project", Home: "/private/codex"}
	lease, err := AcquireReady(args[len(args)-1], scope)
	if err != nil {
		os.Exit(2)
	}
	_, _ = fmt.Fprintln(os.Stdout, "locked")
	_, _ = io.Copy(io.Discard, os.Stdin)
	_ = lease.Close()
	os.Exit(0)
}
