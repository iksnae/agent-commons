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

	"agentcommons/internal/supervision"
)

func TestServicePlanDoesNotInstall(t *testing.T) {
	state := filepath.Join(t.TempDir(), "not-created")
	var out bytes.Buffer
	err := run(context.Background(), []string{"service-plan", "--platform", "darwin", "--binary", "/opt/bin/agent-commons", "--state", state, "--path", "/usr/bin:/opt/bin"}, nil, &out, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	var p supervision.Plan
	if err = json.Unmarshal(out.Bytes(), &p); err != nil || p.Content == "" {
		t.Fatalf("bad plan: %v", err)
	}
	if _, err = os.Stat(state); !os.IsNotExist(err) {
		t.Fatal("plan touched state")
	}
}
