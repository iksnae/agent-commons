// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"agentcommons/internal/core"
)

// An unstamped build must say so. Tests are built by `go test`, which passes no
// -X flag, so this is exactly what a developer's local binary reports.
func TestVersionReportsAnUnstampedBuildHonestly(t *testing.T) {
	if core.Version != core.DevVersion {
		t.Fatalf("test binary is stamped %q; the unstamped default is the thing under test", core.Version)
	}
	for _, args := range [][]string{{"version"}, {"--version"}} {
		var out, errOut bytes.Buffer
		if err := run(context.Background(), args, strings.NewReader(""), &out, &errOut); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if got := out.String(); got != "agent-commons dev\n" {
			t.Fatalf("%v printed %q", args, got)
		}
	}
}

// A stamped build reports the tag it was cut from and nothing else.
func TestVersionReportsTheStampedRelease(t *testing.T) {
	original := core.Version
	core.Version = "v1.2.3"
	t.Cleanup(func() { core.Version = original })
	var out bytes.Buffer
	if err := writeVersion(&out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "agent-commons v1.2.3\n" {
		t.Fatalf("stamped build printed %q", got)
	}
}

// The command surface and the wire DTO must not be able to disagree about which
// release is running. Two independently stamped copies of a version drift; one
// variable, read by both, cannot. Stamping only core's variable — which is what
// -ldflags now targets — has to move both answers together.
func TestVersionAndRuntimeStatusCannotDisagree(t *testing.T) {
	original := core.Version
	core.Version = "v9.9.9-single-source"
	t.Cleanup(func() { core.Version = original })

	var printed bytes.Buffer
	if err := writeVersion(&printed); err != nil {
		t.Fatal(err)
	}
	service, err := core.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	page, err := service.Call("operator", "runtime.status", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	status, ok := page.(core.RuntimeStatusPage)
	if !ok {
		t.Fatalf("runtime.status returned %T", page)
	}
	if got, want := printed.String(), "agent-commons "+status.Build.Version+"\n"; got != want {
		t.Fatalf("`version` printed %q while runtime.status reported %q", got, status.Build.Version)
	}
	if status.Build.Version != core.Version {
		t.Fatalf("runtime.status reported %q, not the stamped %q", status.Build.Version, core.Version)
	}
}
