// SPDX-License-Identifier: MPL-2.0

package integration

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"
)

// Starts an isolated stdio server, never the user's daemon or a model turn.
func codexFixture(t *testing.T, configDir, target string) func(string, any) json.RawMessage {
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
	t.Cleanup(func() { cancel(); _ = in.Close(); _ = command.Wait() })
	encoder, scanner := json.NewEncoder(in), bufio.NewScanner(out)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	id := 0
	request := func(method string, params any) json.RawMessage {
		t.Helper()
		id++
		if err := encoder.Encode(map[string]any{"id": id, "method": method, "params": params}); err != nil {
			t.Fatal(err)
		}
		for scanner.Scan() {
			var event struct {
				ID     int             `json:"id"`
				Result json.RawMessage `json:"result"`
				Error  json.RawMessage `json:"error"`
			}
			if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
				t.Fatal(err)
			}
			if event.ID != id {
				continue
			}
			if len(event.Error) > 0 && string(event.Error) != "null" {
				t.Fatalf("native Codex rejected %s", method)
			}
			return event.Result
		}
		t.Fatalf("native Codex stream closed during %s: %v", method, scanner.Err())
		return nil
	}
	request("initialize", map[string]any{"clientInfo": map[string]string{"name": "agent-commons-test", "version": "0.1.0"}, "capabilities": map[string]bool{"experimentalApi": true}})
	if err := encoder.Encode(map[string]string{"method": "initialized"}); err != nil {
		t.Fatal(err)
	}
	return request
}
