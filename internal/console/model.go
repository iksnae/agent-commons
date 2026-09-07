// SPDX-License-Identifier: MPL-2.0

// Package console is a read-only terminal presentation adapter.
package console

import (
	"context"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

type Snapshot struct {
	Scope         string
	Agents, Tasks []string
}
type Loader func(context.Context) (Snapshot, error)
type loaded struct {
	snapshot Snapshot
	at       time.Time
	err      error
}
type refresh struct{}

type Model struct {
	ctx           context.Context
	load          Loader
	snapshot      Snapshot
	last          time.Time
	err           error
	loading       bool
	width, height int
	viewport      viewport.Model
}

func New(ctx context.Context, load Loader) Model {
	v := viewport.New()
	v.SetWidth(80)
	v.SetHeight(16)
	return Model{ctx: ctx, load: load, width: 80, height: 24, viewport: v, loading: true}
}
func (m Model) Init() tea.Cmd { return m.fetch() }
func (m Model) fetch() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.ctx, 5*time.Second)
		defer cancel()
		snapshot, err := m.load(ctx)
		return loaded{snapshot: snapshot, at: time.Now(), err: err}
	}
}

func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.KeyPressMsg:
		if key.Matches(msg, quitKey) {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.viewport.SetWidth(max(1, msg.Width-2))
		m.viewport.SetHeight(max(1, msg.Height-8))
		m.viewport.SetContent(m.body())
	case loaded:
		m.loading = false
		m.err = msg.err
		if msg.err == nil {
			m.snapshot = msg.snapshot
			m.last = msg.at
		}
		m.viewport.SetContent(m.body())
		return m, tea.Tick(3*time.Second, func(time.Time) tea.Msg { return refresh{} })
	case refresh:
		if !m.loading && m.ctx.Err() == nil {
			m.loading = true
			return m, m.fetch()
		}
	}
	var command tea.Cmd
	m.viewport, command = m.viewport.Update(message)
	return m, command
}
