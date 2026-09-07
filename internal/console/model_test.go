// SPDX-License-Identifier: MPL-2.0

package console

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestSnapshotAndFailureRemainDistinct(t *testing.T) {
	m := New(context.Background(), func(context.Context) (Snapshot, error) { return Snapshot{}, nil })
	updated, _ := m.Update(loaded{snapshot: Snapshot{Scope: "/project", Agents: []string{"lead | claude"}, Tasks: []string{"task | submitted"}}, at: time.Now()})
	m = updated.(Model)
	view := m.View().Content
	if !strings.Contains(view, "lead | claude") || !strings.Contains(view, "RPC connected") {
		t.Fatal(view)
	}
	updated, _ = m.Update(loaded{err: errors.New("offline")})
	view = updated.(Model).View().Content
	if !strings.Contains(view, "STALE") || !strings.Contains(view, "lead | claude") {
		t.Fatal("lost last snapshot or marked it healthy", view)
	}
}

func TestResizeAndQuit(t *testing.T) {
	m := New(context.Background(), nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 20, Height: 5})
	if !strings.Contains(updated.(Model).View().Content, "Resize") {
		t.Fatal("missing small-terminal fallback")
	}
	_, command := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if command == nil {
		t.Fatal("quit unavailable")
	}
	if _, ok := command().(tea.QuitMsg); !ok {
		t.Fatal("quit did not detach UI")
	}
}

func TestPeerTextCannotEmitTerminalControls(t *testing.T) {
	text := clean("lead\x1b[2J\n\u202eevil")
	if strings.ContainsAny(text, "\x1b\n\u202e") {
		t.Fatal("unsafe terminal content", text)
	}
	if len([]rune(clean(strings.Repeat("x", 1000)))) > 513 {
		t.Fatal("unbounded field")
	}
}
