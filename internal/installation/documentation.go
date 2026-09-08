// SPDX-License-Identifier: MPL-2.0

package installation

import (
	"path/filepath"
	"strings"
)

// These guides travel with the binary so its README remains useful offline.
func documentationPayload(path string) bool {
	switch path {
	case "AGENTS.md", "BETA.md", "GATES.md", "PLAN.md", "PRODUCT.md", "PRODUCTION.md",
		"RESEARCH.md", "REVIEW.md", "START-HERE.md", "VALIDATION.md":
		return true
	}
	for _, prefix := range []string{"docs/", "gates/", "integrations/"} {
		if strings.HasPrefix(path, prefix) && filepath.Ext(path) == ".md" {
			return true
		}
	}
	for _, asset := range []string{"docs/assets/agent-commons-hero.png", "docs/assets/agent-commons-icon.png"} {
		if path == asset {
			return true
		}
	}
	return false
}

func payloadDirectory(path string) bool {
	if path == "plugins" {
		return true
	}
	for _, prefix := range []string{"docs", "gates", "integrations", "plugins/agent-commons", "third-party-notices"} {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}
