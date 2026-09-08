// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
)

// The help screen and the catalog are the same list, and a bare invocation now
// renders that screen rather than a second, drifting usage line. These tests
// fail if the screen omits a catalogued command or if the catalog documents a
// command run does not dispatch.
func TestHelpDocumentsEveryCommand(t *testing.T) {
	var out bytes.Buffer
	if err := writeHelp(&out); err != nil {
		t.Fatal(err)
	}
	for _, name := range commandNames() {
		if !bytes.Contains(out.Bytes(), []byte(name)) {
			t.Fatalf("help omits catalogued command %q", name)
		}
	}
}

// Typing the binary's name is a discovery gesture, not a mistake. A bare
// invocation must therefore produce exactly what `help` produces, on stdout,
// with no error: same bytes, same stream, same exit status.
func TestBareInvocationPrintsHelp(t *testing.T) {
	var bare, asked bytes.Buffer
	if err := run(context.Background(), nil, strings.NewReader(""), &bare, io.Discard); err != nil {
		t.Fatalf("bare invocation returned %v", err)
	}
	if err := run(context.Background(), []string{"help"}, strings.NewReader(""), &asked, io.Discard); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bare.Bytes(), asked.Bytes()) {
		t.Fatalf("bare invocation wrote %q, help wrote %q", bare.String(), asked.String())
	}
	if bare.Len() == 0 {
		t.Fatal("bare invocation wrote nothing")
	}
}

func TestEveryDocumentedCommandIsDispatched(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range commandNames() {
		if !bytes.Contains(source, []byte(strconv.Quote(name))) {
			t.Fatalf("help documents %q but run does not dispatch it", name)
		}
	}
}
