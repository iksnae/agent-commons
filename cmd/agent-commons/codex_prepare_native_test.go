// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"agentcommons/internal/core"
)

func TestNativeCodexPreparationCLI(t *testing.T) {
	if os.Getenv("AGENT_COMMONS_CODEX_HOOK") != "1" {
		t.Skip("native Codex preparation is opt-in")
	}
	args, connection := codexPrepareFixture(t)
	const config = `model_provider = "fixture"
[model_providers.fixture]
name = "Offline fixture"
base_url = "http://127.0.0.1:1"
wire_api = "responses"
requires_openai_auth = false
`
	if err := os.WriteFile(filepath.Join(args[3], "config.toml"), []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := run(context.Background(), append([]string{"codex-prepare"}, args...), nil, &output, io.Discard); err != nil {
		t.Fatal(err)
	}
	var report codexPreparationReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil || !report.Prepared || report.ThreadID == "" {
		t.Fatal("native CLI did not prepare thread", err)
	}
	output.Reset()
	if err := run(context.Background(), append([]string{"codex-resume-check"}, args...), nil, &output, io.Discard); err != nil {
		t.Fatal("native CLI could not verify saved resume", err)
	}
	var resumed codexResumeReport
	if err := json.Unmarshal(output.Bytes(), &resumed); err != nil || !resumed.Verified || resumed.ThreadID != report.ThreadID || resumed.Binding != report.Binding {
		t.Fatal("native CLI resumed a different binding", err)
	}
	peers, err := rpcCall[[]core.Session](context.Background(), connection.client, "sessions.list", struct{}{})
	if err != nil || len(peers) != 1 || peers[0].Attachment.NativeID != "" {
		t.Fatal("preparation attached a role", err)
	}
	t.Log("native CLI prepared and verified resume of the same scoped thread in disposable state without a model turn or role attachment")
}
