// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// An unstamped build must say so. Tests are built by `go test`, which passes no
// -X flag, so this is exactly what a developer's local binary reports.
func TestVersionReportsAnUnstampedBuildHonestly(t *testing.T) {
	if version != devVersion {
		t.Fatalf("test binary is stamped %q; the unstamped default is the thing under test", version)
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
	original := version
	version = "v1.2.3"
	t.Cleanup(func() { version = original })
	var out bytes.Buffer
	if err := writeVersion(&out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "agent-commons v1.2.3\n" {
		t.Fatalf("stamped build printed %q", got)
	}
}
