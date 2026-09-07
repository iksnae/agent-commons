// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"fmt"
	"strings"
)

// Managed children get only launch essentials and their own provider's auth.
// This does not isolate files in HOME, keychain access, or same-provider roles.
func managedEnvironment(runtime string, inherited []string) ([]string, error) {
	allowed := map[string]bool{}
	for _, key := range []string{"PATH", "HOME", "TMPDIR", "LANG", "LC_ALL", "LC_CTYPE", "TZ", "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "all_proxy", "no_proxy", "SSL_CERT_FILE", "SSL_CERT_DIR", "NODE_EXTRA_CA_CERTS"} {
		allowed[key] = true
	}
	var auth []string
	switch runtime {
	case "claude":
		auth = []string{"ANTHROPIC_API_KEY", "CLAUDE_CODE_OAUTH_TOKEN", "CLAUDE_CONFIG_DIR"}
	case "codex":
		auth = []string{"OPENAI_API_KEY", "CODEX_API_KEY", "CODEX_ACCESS_TOKEN", "CODEX_HOME", "CODEX_SQLITE_HOME", "CODEX_CA_CERTIFICATE"}
	default:
		return nil, fmt.Errorf("unsupported managed runtime %q", runtime)
	}
	for _, key := range auth {
		allowed[key] = true
	}
	result := make([]string, 0, len(allowed))
	for _, entry := range inherited {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if value != "" && !allowed[key] && providerSelector(runtime, key) {
			return nil, fmt.Errorf("managed %s does not support inherited %s; explicit provider profile required", runtime, key)
		}
		if allowed[key] {
			result = append(result, entry)
		}
	}
	return result, nil
}

func providerSelector(runtime, key string) bool {
	if runtime == "codex" {
		return strings.HasPrefix(key, "OPENAI_")
	}
	return strings.HasPrefix(key, "ANTHROPIC_") || strings.HasPrefix(key, "CLAUDE_CODE_USE_") || strings.HasPrefix(key, "CLAUDE_CODE_OAUTH_")
}
