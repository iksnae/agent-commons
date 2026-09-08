// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"

	"agentcommons/internal/core"
)

// consoleSnapshotJSON is the `console --once` (and non-terminal) output
// contract. It belongs to the CLI, not to the terminal UI.
//
// It exists because that output used to be `console.Snapshot` — the Bubble Tea
// model's own view state — marshalled straight to stdout. That inverted
// ownership: any change to how the UI holds its rows was a change to a machine
// contract, and a UI field that came to hold a `core.Session` or `core.Task`
// would have silently widened this document to core's full shape, publishing
// fields such as Attachment.LeaseID and Session.Instructions that the console
// has no business emitting. The two are now separate types on purpose. Do not
// re-point the writer at the UI model to save a struct.
//
// The field set is frozen. Changes are ADDITIVE ONLY, forever: a new field may
// be appended, but no existing field may be renamed, retyped, reordered or
// removed, and no field may change from present to absent. Consumers parse this
// document by key; anything else silently breaks scripts that already read it.
// The JSON tags are written out explicitly rather than inherited from the Go
// field names so that renaming a Go field cannot rename a wire key by accident.
type consoleSnapshotJSON struct {
	Scope  string   `json:"Scope"`
	Agents []string `json:"Agents"`
	Tasks  []string `json:"Tasks"`
}

// consoleSnapshotJSONLoader produces the CLI contract document. The writer is
// typed on this, not on console.Loader, so the UI model cannot reach stdout.
type consoleSnapshotJSONLoader func(context.Context) (consoleSnapshotJSON, error)

// Row formats for the emitted document. They are the console's wire format, not
// its display format; a later change to how the terminal UI renders a row must
// not reach through to here.
const (
	consoleAgentRowFormat = "%s / %s | %s | %s | %s"
	consoleTaskRowFormat  = "%s | %s | revision %d | %s"
)

func newConsoleSnapshotJSON(scope string, peers []core.Session, tasks []core.Task, now time.Time) consoleSnapshotJSON {
	document := consoleSnapshotJSON{Scope: scope}
	for _, peer := range peers {
		state := "not attached"
		if peer.Busy {
			state = "managed delivery running"
		} else if peer.Attachment.ExpiresAt > now.Unix() {
			state = "attachment lease current (reachability unverified)"
		}
		document.Agents = append(document.Agents, fmt.Sprintf(consoleAgentRowFormat, peer.Name, peer.Role, peer.Runtime, peer.ID, state))
	}
	for _, task := range tasks {
		document.Tasks = append(document.Tasks, fmt.Sprintf(consoleTaskRowFormat, task.ID, task.Status, task.Revision, task.Title))
	}
	sort.Strings(document.Agents)
	sort.Strings(document.Tasks)
	return document
}

func loadConsoleSnapshotJSON(ctx context.Context, connection projectConnection) (consoleSnapshotJSON, error) {
	scope := connection.config.Target
	if err := connection.verifyIdentity(ctx); err != nil {
		return consoleSnapshotJSON{Scope: scope}, err
	}
	peers, err := rpcCall[[]core.Session](ctx, connection.client, "sessions.list", struct{}{})
	if err != nil {
		return consoleSnapshotJSON{Scope: scope}, err
	}
	tasks, err := rpcCall[[]core.Task](ctx, connection.client, "tasks.list", struct{}{})
	if err != nil {
		return consoleSnapshotJSON{Scope: scope}, err
	}
	return newConsoleSnapshotJSON(scope, peers, tasks, time.Now()), nil
}

func writeConsoleSnapshot(ctx context.Context, load consoleSnapshotJSONLoader, out io.Writer) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	document, err := load(ctx)
	if err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(document)
}
