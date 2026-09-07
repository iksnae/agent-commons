// SPDX-License-Identifier: MPL-2.0

package console

import (
	"fmt"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func clean(text string) string {
	runes := []rune(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return ' '
		}
		return r
	}, text))
	if len(runes) > 512 {
		return string(runes[:512]) + "…"
	}
	return string(runes)
}

func (m Model) body() string {
	var b strings.Builder
	for _, section := range []struct {
		name string
		rows []string
	}{{"Registered agents", m.snapshot.Agents}, {"Tasks", m.snapshot.Tasks}} {
		fmt.Fprintf(&b, "%s (%d)\n", section.name, len(section.rows))
		if len(section.rows) == 0 {
			b.WriteString("  None in this snapshot.\n")
		}
		for i, row := range section.rows {
			if i == 200 {
				b.WriteString("  Display limited to 200 rows.\n")
				break
			}
			b.WriteString(lipgloss.NewStyle().Width(max(1, m.width-2)).Render(clean(row)) + "\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (m Model) View() tea.View {
	if m.width < 40 || m.height < 12 {
		v := tea.NewView("Resize to at least 40x12. q: quit console")
		v.AltScreen = true
		return v
	}
	status := "Connecting…"
	if !m.last.IsZero() {
		status = "RPC connected | snapshot " + m.last.Format("15:04:05")
	}
	if m.err != nil {
		status = "DISCONNECTED / STALE | refresh failed; inspect doctor"
	}
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6")).Render("Agent Commons / read-only console")
	content := header + "\n" + status + "\nScope: " + clean(m.snapshot.Scope) + "\n\n" + m.viewport.View() + "\n\nRegistered is not reachable; accepted is not shipped.\n" + keyHelp()
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}
