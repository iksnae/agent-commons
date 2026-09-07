// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"agentcommons/internal/core"
)

func TestManagedEnvironmentSelectsOnlyHarnessCredentials(t *testing.T) {
	input := []string{"PATH=/bin", "HOME=/fixture", "LANG=C", "HTTPS_PROXY=http://proxy.invalid", "OPENAI_API_KEY=codex-fixture", "CODEX_HOME=/codex", "ANTHROPIC_API_KEY=claude-fixture", "CLAUDE_CODE_OAUTH_TOKEN=oauth-fixture", "CLAUDE_CONFIG_DIR=/claude", "AWS_SECRET_ACCESS_KEY=unrelated", "GITHUB_TOKEN=unrelated", "AGENT_COMMONS_TOKEN=operator", "NODE_OPTIONS=--require=/untrusted", "BASH_ENV=/untrusted", "DYLD_INSERT_LIBRARIES=/untrusted", "LD_PRELOAD=/untrusted", "MALFORMED"}
	codexOverrides := []string{"CODEX_API_KEY=exec-fixture", "CODEX_ACCESS_TOKEN=access-fixture", "CODEX_SQLITE_HOME=/state-fixture", "CODEX_CA_CERTIFICATE=/ca-fixture"}
	input = append(input, codexOverrides...)
	for _, harness := range []string{"claude", "codex"} {
		t.Run(harness, func(t *testing.T) {
			want := append([]string{}, input[:4]...)
			if harness == "codex" {
				want = append(want, input[4:6]...)
				want = append(want, codexOverrides...)
			} else {
				want = append(want, input[6:9]...)
			}
			got, err := managedEnvironment(harness, input)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatal("managed environment crossed allowlist boundary", err)
			}
		})
	}
}

func TestManagedEnvironmentRefusesUnsupportedProviderRouting(t *testing.T) {
	for _, test := range []struct{ harness, key string }{
		{"claude", "CLAUDE_CODE_USE_VERTEX"}, {"claude", "ANTHROPIC_FEDERATION_RULE_ID"},
		{"claude", "ANTHROPIC_PROFILE"}, {"claude", "ANTHROPIC_MODEL"},
		{"claude", "CLAUDE_CODE_OAUTH_REFRESH_TOKEN"}, {"codex", "OPENAI_BASE_URL"},
		{"codex", "OPENAI_FEDERATION_RULE_ID"}, {"codex", "OPENAI_IDENTITY_TOKEN_FILE"},
	} {
		harness, key := test.harness, test.key
		got, err := managedEnvironment(harness, []string{key + "=private-endpoint-fixture"})
		if err == nil || got != nil || !strings.Contains(err.Error(), key) || strings.Contains(err.Error(), "private-endpoint-fixture") {
			t.Fatal("provider routing silently changed or diagnostic exposed value")
		}
	}
}

func TestManagedChildrenDoNotInheritUnrelatedCredentials(t *testing.T) {
	for _, harness := range []string{"claude", "codex"} {
		t.Run(harness, func(t *testing.T) {
			t.Setenv("OPENAI_BASE_URL", "")
			for _, key := range []string{"ANTHROPIC_BASE_URL", "ANTHROPIC_AUTH_TOKEN", "CLAUDE_CODE_USE_BEDROCK", "CLAUDE_CODE_USE_VERTEX", "CLAUDE_CODE_USE_FOUNDRY", "CLAUDE_CODE_USE_MANTLE"} {
				t.Setenv(key, "")
			}
			t.Setenv("COMMONS_UNRELATED_SECRET", "fixture")
			t.Setenv("AGENT_COMMONS_TOKEN", "operator-fixture")
			t.Setenv("OPENAI_API_KEY", "codex-fixture")
			t.Setenv("CODEX_API_KEY", "exec-fixture")
			t.Setenv("CODEX_ACCESS_TOKEN", "access-fixture")
			t.Setenv("ANTHROPIC_API_KEY", "claude-fixture")
			body := `cat >/dev/null
test -z "${COMMONS_UNRELATED_SECRET+x}" || exit 20
test -z "${AGENT_COMMONS_TOKEN+x}" || exit 21
`
			if harness == "claude" {
				body += `test -z "${OPENAI_API_KEY+x}" || exit 22
test -z "${CODEX_API_KEY+x}" || exit 24
test -z "${CODEX_ACCESS_TOKEN+x}" || exit 25
test "$ANTHROPIC_API_KEY" = claude-fixture || exit 23
printf '%s' '{"session_id":"fixture","result":"isolated environment"}'
`
			} else {
				body += `test -z "${ANTHROPIC_API_KEY+x}" || exit 22
test "$OPENAI_API_KEY" = codex-fixture || exit 23
test "$CODEX_API_KEY" = exec-fixture || exit 24
test "$CODEX_ACCESS_TOKEN" = access-fixture || exit 25
printf '%s\n' '{"type":"thread.started","thread_id":"fixture"}' '{"type":"item.completed","item":{"type":"agent_message","text":"isolated environment"}}' '{"type":"turn.completed"}'
`
			}
			fixture(t, harness, body)
			id, output, err := (CLI{}).Run(context.Background(), core.Session{ID: "role", Mode: "managed", Runtime: harness, Target: t.TempDir()}, core.Delivery{})
			if err != nil || id != "fixture" || output != "isolated environment" {
				t.Fatal("managed child environment contract failed", err)
			}
		})
	}
}
