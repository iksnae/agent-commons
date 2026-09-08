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

// initRuntimeAllowlist is deliberately narrower than harness.Catalog(), the
// same pattern as the --runtime discovery flag at main.go:102-105: a literal
// allowlist commented with its reason, not a new field on harness.Capability.
// "Has an adapter been built" (what Capability records) and "is this runtime
// deferred by product decision" are different kinds of fact, and the second
// can flip with no code change. Hermes is explicitly deferred (START-HERE.md:
// 27-35) and Pi remains scoped behind the core Claude/Codex team delivery
// (PLAN.md:103-105); `init` writes its --runtime value as project metadata,
// so it must not silently accept a runtime the product has not enrolled yet.
// This allowlist enumerates what init accepts (fails closed) rather than
// subtracting deferred runtimes from the catalog (fails open): a future
// fifth catalog runtime is rejected by default here until someone decides
// otherwise, instead of being silently admitted.
var initRuntimeAllowlist = []string{"claude", "codex", "pi"}

// initRuntimeVocabulary is the human-readable phrase for initRuntimeAllowlist,
// used by both init's help text and its acceptance check so they cannot
// drift from each other the way init's help text once drifted from the
// harness catalog.
func initRuntimeVocabulary() string {
	return joinWithOr(initRuntimeAllowlist)
}

// initRuntimeAllowed reports whether id is a runtime `init` accepts. It is
// intentionally not harness.CanAttach/harness.Lookup: check-in and onboarding
// correctly use the full catalog (Hermes and Pi may check in manually per
// PLAN.md:103-104), but init's --runtime is project metadata gated on the
// narrower, deliberately-scoped set above.
func initRuntimeAllowed(id string) bool {
	for _, allowed := range initRuntimeAllowlist {
		if allowed == id {
			return true
		}
	}
	return false
}
