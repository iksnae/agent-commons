// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"agentcommons/internal/core"
)

func TestUnsupportedRoutingStopsBeforePreparationAndExecution(t *testing.T) {
	for harness, key := range map[string]string{"claude": "ANTHROPIC_BASE_URL", "codex": "OPENAI_BASE_URL"} {
		t.Run(harness, func(t *testing.T) {
			t.Setenv(key, "private-endpoint-fixture")
			marker := filepath.Join(t.TempDir(), "started")
			fixture(t, harness, "touch "+strconv.Quote(marker)+"\nexit 1\n")
			cli := CLI{
				PrepareCodex: func(context.Context, core.Session) (string, error) {
					t.Fatal("prepared before routing validation")
					return "", nil
				},
				Checkpoint: func(string, string) error {
					t.Fatal("checkpointed unsupported provider run")
					return nil
				},
			}
			id, output, err := cli.Run(context.Background(), core.Session{ID: "role", Mode: "managed", Runtime: harness, Target: t.TempDir()}, core.Delivery{})
			if err == nil || !strings.Contains(err.Error(), "explicit provider profile required") || id != "" || output != "" {
				t.Fatal("unsupported routing did not stop dispatch", err)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatal("provider process started", err)
			}
		})
	}
}
