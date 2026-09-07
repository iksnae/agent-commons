// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

type piFixture struct {
	binary, directory string
	env               []string
}

func (f piFixture) command(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, f.binary, args...)
	cmd.Dir, cmd.Env = f.directory, f.env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = time.Second
	return cmd
}

func (f piFixture) run(t *testing.T, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if output, err := f.command(ctx, args...).CombinedOutput(); err != nil {
		t.Fatalf("Pi %v: %v (%d output bytes withheld)", args, err, len(output))
	}
}

func (f piFixture) inspectSession(t *testing.T, sessionArgs ...string) map[string]json.RawMessage {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	args := []string{"--mode", "rpc", "--offline", "--no-tools", "--no-context-files", "--no-prompt-templates", "--no-themes"}
	cmd := f.command(ctx, append(args, sessionArgs...)...)
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = io.Discard
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { cancel(); input.Close(); cmd.Wait() }()
	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 4096), 2<<20)
	responses := map[string]json.RawMessage{}
	for _, method := range []string{"get_state", "get_messages"} {
		if err = json.NewEncoder(input).Encode(map[string]string{"type": method, "id": method}); err != nil {
			t.Fatal(err)
		}
		found := false
		for scanner.Scan() {
			var event struct {
				Type, ID string
				Success  bool
				Data     json.RawMessage
			}
			if err = json.Unmarshal(scanner.Bytes(), &event); err != nil {
				t.Fatal("invalid Pi JSON", err)
			}
			if event.Type == "agent_start" {
				t.Fatal("unexpected model turn")
			}
			if event.Type != "response" || event.ID != method {
				continue
			}
			if !event.Success {
				t.Fatal("Pi metadata query failed", method)
			}
			responses[method], found = event.Data, true
			break
		}
		if !found {
			t.Fatal("Pi response missing", method, scanner.Err())
		}
	}
	return responses
}
