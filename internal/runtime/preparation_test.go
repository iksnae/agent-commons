// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"agentcommons/internal/core"
)

func TestManagedPreparationMustCheckpointBeforeModelExecution(t *testing.T) {
	for _, fail := range []string{"prepare", "empty-identity", "missing-checkpoint", "checkpoint", "none"} {
		t.Run(fail, func(t *testing.T) {
			prepared, saved := false, false
			marker := filepath.Join(t.TempDir(), "model-started")
			t.Setenv("COMMONS_TEST_MODEL_MARKER", marker)
			fixture(t, "codex", `cat >/dev/null
touch "$COMMONS_TEST_MODEL_MARKER"
case "$*" in *resume*prepared-root*) ;; *) exit 4;; esac
printf '%s\n' '{"type":"thread.started","thread_id":"prepared-root"}' '{"type":"item.completed","item":{"type":"agent_message","text":"executed"}}' '{"type":"turn.completed"}'
`)
			cli := CLI{
				PrepareCodex: func(context.Context, core.Session) (string, error) {
					prepared = true
					if fail == "prepare" {
						return "partial-root", errors.New("not ready")
					}
					if fail == "empty-identity" {
						return "", nil
					}
					return "prepared-root", nil
				},
				Checkpoint: func(delivery, id string) error {
					if !prepared || delivery != "delivery" || id != "prepared-root" {
						t.Fatal("bad checkpoint")
					}
					if fail == "checkpoint" {
						return errors.New("disk unavailable")
					}
					saved = true
					return nil
				},
			}
			if fail == "missing-checkpoint" {
				cli.Checkpoint = nil
			}
			id, out, err := cli.Run(context.Background(), core.Session{ID: "role", Mode: "managed", Runtime: "codex", Target: t.TempDir()}, core.Delivery{ID: "delivery"})
			if fail == "missing-checkpoint" && prepared {
				t.Fatal("created native root without a checkpoint sink")
			}
			if fail == "none" {
				if err != nil || !saved || id != "prepared-root" || out != "executed" {
					t.Fatal("prepared run failed", err)
				}
			} else if err == nil || out != "" || id != "" || saved {
				t.Fatal("incomplete preparation ran or exposed a result")
			}
			if _, statErr := os.Stat(marker); fail != "none" && !os.IsNotExist(statErr) {
				t.Fatal("model process started before durable checkpoint", statErr)
			}
		})
	}
}

func TestPreparationPreservesExistingNativeBinding(t *testing.T) {
	cli := CLI{
		PrepareCodex: func(context.Context, core.Session) (string, error) {
			t.Fatal("prepared a replacement for an existing binding")
			return "", nil
		},
		Checkpoint: func(string, string) error {
			t.Fatal("checkpointed an already-bound session")
			return nil
		},
	}
	session := core.Session{ID: "role", Runtime: "codex", RuntimeSessionID: "existing-root"}
	got, err := cli.prepareSession(context.Background(), session, core.Delivery{})
	if err != nil || got.RuntimeSessionID != session.RuntimeSessionID {
		t.Fatal("existing binding changed", err)
	}
}
