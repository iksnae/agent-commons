// SPDX-License-Identifier: MPL-2.0

package integration

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"agentcommons/internal/codexrpc"
)

// Starts an isolated stdio server, never the user's daemon or a model turn.
func codexFixture(t *testing.T, configDir, target string) func(string, any) json.RawMessage {
	request, _ := codexFixtureWithStop(t, configDir, target)
	return request
}

func codexFixtureWithStop(t *testing.T, configDir, target string) (func(string, any) json.RawMessage, func()) {
	t.Helper()
	codex, err := exec.LookPath("codex")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	command := exec.CommandContext(ctx, codex, "app-server", "--stdio", "--config", `model_provider="fixture"`, "--config", `model_providers.fixture={name="Offline fixture",base_url="http://127.0.0.1:1",wire_api="responses",requires_openai_auth=false}`)
	command.Dir = target
	command.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME"), "TMPDIR=" + os.Getenv("TMPDIR"), "CODEX_HOME=" + configDir}
	command.WaitDelay = time.Second
	command.Stderr = io.Discard
	in, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := command.StdoutPipe()
	if err != nil {
		_ = in.Close()
		t.Fatal(err)
	}
	if err = command.Start(); err != nil {
		_ = in.Close()
		_ = out.Close()
		t.Fatal(err)
	}
	client := codexrpc.New(codexrpc.Pipes(in, out))
	var once sync.Once
	stop := func() { once.Do(func() { cancel(); _ = client.Close(); _ = command.Wait() }) }
	t.Cleanup(stop)
	request := func(method string, params any) json.RawMessage {
		t.Helper()
		result, err := client.Call(ctx, method, params)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	request("initialize", map[string]any{"clientInfo": map[string]string{"name": "agent-commons-test", "version": "0.1.0"}, "capabilities": map[string]bool{"experimentalApi": true}})
	if err := client.Notify(ctx, "initialized"); err != nil {
		t.Fatal(err)
	}
	return request, stop
}
