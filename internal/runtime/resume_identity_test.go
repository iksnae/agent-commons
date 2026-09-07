// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"context"
	"testing"

	"agentcommons/internal/core"
)

func TestManagedResumeRejectsChangedNativeIdentity(t *testing.T) {
	for _, runtime := range []string{"claude", "codex"} {
		t.Run(runtime, func(t *testing.T) {
			body := `cat >/dev/null
printf '%s' '{"session_id":"different-root","result":"untrusted result"}'
`
			if runtime == "codex" {
				body = `cat >/dev/null
printf '%s\n' '{"type":"thread.started","thread_id":"different-root"}' '{"type":"item.completed","item":{"type":"agent_message","text":"untrusted result"}}' '{"type":"turn.completed"}'
`
			}
			fixture(t, runtime, body)
			id, out, err := (CLI{}).Run(context.Background(), core.Session{Mode: "managed", Runtime: runtime, Target: t.TempDir(), RuntimeSessionID: "bound-root"}, core.Delivery{})
			if err == nil || id != "" || out != "" {
				t.Fatal("accepted output or identity from a different native root")
			}
		})
	}
}

func TestCodexResultRejectsConflictingThreadEvents(t *testing.T) {
	raw := []byte("{\"type\":\"thread.started\",\"thread_id\":\"one\"}\n{\"type\":\"thread.started\",\"thread_id\":\"two\"}\n{\"type\":\"item.completed\",\"item\":{\"type\":\"agent_message\",\"text\":\"mixed\"}}\n{\"type\":\"turn.completed\"}\n")
	if id, out, err := parse("codex", raw); err == nil || id != "" || out != "" {
		t.Fatal("accepted mixed native thread stream")
	}
}
