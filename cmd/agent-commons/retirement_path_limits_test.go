// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The two enrollment flags retirementFiles cannot answer for. Both are recorded
// as known limits in that function's comment, and a recorded limit is only worth
// something if it is the limit that actually exists -- so these assert the real
// behaviour rather than restating the prose.
//
// If either of these ever fails, the comment is now wrong and must be corrected
// with the code; that is the point of pinning them.

// enroll --id honours an explicit identity, so the files ARE at that identity's
// digest. The guess from target/name/role does not match it, so the report says
// it cannot locate them: a false negative, and the notice must not blame an
// adoption that never happened.
func TestRetireCannotLocateFilesForAnExplicitlyIdentifiedEnrollment(t *testing.T) {
	state := onboardingService(t)
	target := t.TempDir()
	const explicit = "operator-chosen-id"
	if err := run(context.Background(), []string{"enroll", "--json", "--state", state,
		"--target", target, "--name", "exp-alpha", "--role", "builder", "--id", explicit},
		nil, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	// The files really are there, under the digest of the explicit identity.
	real := filepath.Join(state, "connection-"+identityDigest(explicit)+".json")
	if _, err := os.Lstat(real); err != nil {
		t.Fatalf("fixture did not write the connection at %s: %v", real, err)
	}

	document := retirementDocument(t, "retire", "--json", "--state", state,
		"--id", explicit, "--evidence", "Withdrawing an explicitly identified role.")

	if document["connection"] != "" {
		t.Fatalf("report located files it cannot derive: %q", document["connection"])
	}
	// The recorded limit: a false negative, whose stated cause must include the
	// explicit --id and not name adoption alone.
	if !strings.Contains(document["notice"], "explicit --id") {
		t.Fatalf("notice blames only adoption for a case caused by --id: %q", document["notice"])
	}
}

// enroll --config puts the files somewhere else entirely. The identity still
// matches the guess, so the report names state-directory paths that are not
// where the files live. This case is NOT detected, and the located notice says
// the paths are the default location for exactly this reason.
func TestRetireNamesDefaultPathsForAConfigRelocatedEnrollment(t *testing.T) {
	state := onboardingService(t)
	target := t.TempDir()
	elsewhere := t.TempDir()
	// enroll requires a private parent directory for an explicit --config.
	if err := os.Chmod(elsewhere, 0o700); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(elsewhere, "somewhere-else.json")
	if err := run(context.Background(), []string{"enroll", "--json", "--state", state,
		"--target", target, "--name", "exp-alpha", "--role", "builder", "--config", config},
		nil, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	var written connectionConfig
	if err := privateRead(config, &written); err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(written.TokenFile) != elsewhere {
		t.Fatalf("fixture did not relocate the credential: %s", written.TokenFile)
	}

	document := retirementDocument(t, "retire", "--json", "--state", state,
		"--id", written.Identity, "--evidence", "Withdrawing a relocated connection.")

	// The known gap: located is true and the named paths are not the real ones.
	if document["connection"] == "" {
		t.Fatal("the --config case is now detected; update retirementFiles' recorded limits")
	}
	if document["connection"] == config {
		t.Fatal("the --config path is now resolved; update retirementFiles' recorded limits")
	}
	// Since the paths can be wrong, the notice must not present them as certain.
	if !strings.Contains(document["notice"], "by default") || !strings.Contains(document["notice"], "--config") {
		t.Fatalf("notice presents derived paths as authoritative: %q", document["notice"])
	}
}
