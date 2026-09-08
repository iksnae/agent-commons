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
// (init omitted hermes; onboarding was current only by luck). onboarding is
// now derived from harness.Catalog() via runtimeVocabulary; this test fails
// the moment a new catalog entry is not reflected in onboarding's actual
// flag help text, not merely in the helper function.
func TestRuntimeVocabularyListsEveryCatalogEntry(t *testing.T) {
	vocabulary := runtimeVocabulary()
	for _, capability := range harness.Catalog() {
		if !strings.Contains(vocabulary, capability.ID) {
			t.Fatalf("runtime vocabulary %q omits catalog entry %q", vocabulary, capability.ID)
		}
	}
}

// init's --runtime is scoped narrower than the full catalog on purpose (see
// runtime_vocabulary.go's initRuntimeAllowlist doc comment): Hermes is
// deferred, Pi remains behind core-team delivery. Its help text must list
// exactly initRuntimeAllowlist, matched against the command's real --help
// output so the two cannot drift the way the usage line and help screen once
// drifted (commands_test.go).
func TestInitRuntimeFlagHelpMatchesCatalog(t *testing.T) {
	var out, errOut bytes.Buffer
	_ = runInit(context.Background(), []string{"--help"}, &out, &errOut)
	help := errOut.String()
	if help == "" {
		t.Fatal("no flag help text captured")
	}
	for _, allowed := range initRuntimeAllowlist {
		if !strings.Contains(help, allowed) {
			t.Fatalf("init help omits allowed runtime %q:\n%s", allowed, help)
		}
	}
	for _, capability := range harness.Catalog() {
		if initRuntimeAllowed(capability.ID) {
			continue
		}
		if strings.Contains(help, capability.ID) {
			t.Fatalf("init help advertises %q, which init does not accept:\n%s", capability.ID, help)
		}
	}
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

// TestInitRejectsDeferredRuntimeWhileCheckInAcceptsIt pins init and
// check-in as deliberately different, per the architect's ruling: init's
// --runtime is project metadata gated on a narrow allowlist (Hermes
// deferred, START-HERE.md:27-35; Pi behind core delivery, PLAN.md:103-105),
// while check-in's --runtime governs explicit manual attachment, which
// PLAN.md:103-104 sanctions for both Pi and Hermes today. A future edit that
// quietly reconciles the two by widening or narrowing one to match the
// other should fail this test.
func TestInitRejectsDeferredRuntimeWhileCheckInAcceptsIt(t *testing.T) {
	if initRuntimeAllowed("hermes") {
		t.Fatal("init must not accept the deferred hermes runtime")
	}
	var out, errOut bytes.Buffer
	home := t.TempDir()
	t.Setenv("HOME", home)
	target := t.TempDir()
	err := runInit(context.Background(), []string{"--target", target, "--name", "lead", "--role", "lead", "--runtime", "hermes"}, &out, &errOut)
	if err == nil {
		t.Fatal("init --runtime hermes was accepted")
	}
	if !strings.Contains(err.Error(), "runtime must be") {
		t.Fatalf("init did not name the runtime problem: %v", err)
	}

	if !harness.CanAttach("hermes") {
		t.Fatal("harness.CanAttach must still accept hermes for check-in")
	}
	if _, err := resolveCheckIn(onboardingOptions{Config: "/irrelevant", Runtime: "hermes", NativeSession: "native"}); err != nil {
		t.Fatalf("check-in rejected hermes, which PLAN.md:103-104 sanctions: %v", err)
	}
}
