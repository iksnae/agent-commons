// SPDX-License-Identifier: MPL-2.0

package main

import (
	"strings"

	"agentcommons/internal/harness"
)

// runtimeVocabulary lists every harness.Catalog() runtime ID as one
// human-readable phrase ("claude, codex, pi or hermes"), so flag help text
// cannot silently drift from the catalog that actually implements it.
func runtimeVocabulary() string {
	catalog := harness.Catalog()
	ids := make([]string, len(catalog))
	for i, capability := range catalog {
		ids[i] = capability.ID
	}
	return joinWithOr(ids)
}

func joinWithOr(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " or " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + " or " + items[len(items)-1]
	}
}
