// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestRedirectedConsoleIsBoundedJSON(t *testing.T) {
	var output bytes.Buffer
	if consoleTerminal(&output) {
		t.Fatal("buffer mistaken for terminal")
	}
	calls := 0
	load := func(ctx context.Context) (consoleSnapshotJSON, error) {
		calls++
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("unbounded snapshot call")
		}
		return consoleSnapshotJSON{Scope: "project", Agents: []string{"peer\x1b[2J"}}, nil
	}
	if err := writeConsoleSnapshot(context.Background(), load, &output); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || !json.Valid(output.Bytes()) || bytes.ContainsRune(output.Bytes(), '\x1b') {
		t.Fatal("unsafe/non-JSON console output", output.String())
	}
}

func TestFailedStaticSnapshotDoesNotEmitSuccess(t *testing.T) {
	var output bytes.Buffer
	err := writeConsoleSnapshot(context.Background(), func(context.Context) (consoleSnapshotJSON, error) {
		return consoleSnapshotJSON{}, errors.New("offline")
	}, &output)
	if err == nil || output.Len() != 0 {
		t.Fatal("failure presented as snapshot")
	}
}
