// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"agentcommons/internal/harness"
)

// These are guard tests in the spirit of commands_test.go: the flag help
// text for init and onboarding once hand-restated the runtime list and drifted
// (init omitted hermes; onboarding was current only by luck). Both are now
// derived from harness.Catalog() via runtimeVocabulary; these tests fail the
// moment a new catalog entry is not reflected in the actual flag help text
// those commands print, not merely in the helper function.
func TestRuntimeVocabularyListsEveryCatalogEntry(t *testing.T) {
	vocabulary := runtimeVocabulary()
	for _, capability := range harness.Catalog() {
		if !strings.Contains(vocabulary, capability.ID) {
			t.Fatalf("runtime vocabulary %q omits catalog entry %q", vocabulary, capability.ID)
		}
	}
}

func TestInitRuntimeFlagHelpMatchesCatalog(t *testing.T) {
	var out, errOut bytes.Buffer
	_ = runInit(context.Background(), []string{"--help"}, &out, &errOut)
	assertHelpListsCatalog(t, errOut.String())
}

func TestOnboardingRuntimeFlagHelpMatchesCatalog(t *testing.T) {
	var errOut bytes.Buffer
	_, _ = parseOnboarding([]string{"enroll", "--help"}, &errOut)
	assertHelpListsCatalog(t, errOut.String())
}

func assertHelpListsCatalog(t *testing.T, help string) {
	t.Helper()
	if help == "" {
		t.Fatal("no flag help text captured")
	}
	for _, capability := range harness.Catalog() {
		if !strings.Contains(help, capability.ID) {
			t.Fatalf("flag help omits catalog runtime %q:\n%s", capability.ID, help)
		}
	}
}
