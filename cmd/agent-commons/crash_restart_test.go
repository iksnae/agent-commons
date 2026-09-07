// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"agentcommons/internal/core"
)

func TestServiceCrashHelper(t *testing.T) {
	state := os.Getenv("AGENT_COMMONS_TEST_SERVICE_STATE")
	if state == "" {
		return
	}
	if err := run(context.Background(), []string{"serve", "--state", state}, nil, io.Discard, io.Discard); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func startCrashService(t *testing.T, state string) (*exec.Cmd, rpcClient) {
	t.Helper()
	command := exec.Command(os.Args[0], "-test.run=^TestServiceCrashHelper$")
	command.Env = append(os.Environ(), "AGENT_COMMONS_TEST_SERVICE_STATE="+state)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { command.Process.Kill(); command.Wait() })
	client := rpcClient{socket: filepath.Join(state, "service.sock")}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		client.token, _ = readToken(filepath.Join(state, "operator.token"))
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		_, err := rpcCall[json.RawMessage](ctx, client, "sessions.list", struct{}{})
		cancel()
		if err == nil {
			return command, client
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("service failed to become reachable")
	return nil, rpcClient{}
}

func TestCrashRestartPreservesRegistryAndRecoversSocket(t *testing.T) {
	state, err := os.MkdirTemp("", "ac-crash-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(state) })
	first, client := startCrashService(t, state)
	_, err = rpcCall[json.RawMessage](context.Background(), client, "sessions.register", core.Session{
		ID: "persistent", Target: t.TempDir(), Name: "lead", Role: "lead", Team: "test", Runtime: "manual", Mode: "manual",
	})
	if err != nil {
		t.Fatal(err)
	}
	// A second process cannot acquire the lock or clean the live socket.
	if err = run(context.Background(), []string{"serve", "--state", state}, nil, io.Discard, io.Discard); err == nil {
		t.Fatal("duplicate service started")
	}
	if err = first.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = first.Wait()
	if _, err = os.Lstat(client.socket); err != nil {
		t.Fatal("crash did not leave socket fixture")
	}
	_, restarted := startCrashService(t, state)
	peers, err := rpcCall[[]core.Session](context.Background(), restarted, "sessions.list", struct{}{})
	if err != nil || len(peers) != 1 || peers[0].ID != "persistent" {
		t.Fatalf("registry lost: %v %+v", err, peers)
	}
}
