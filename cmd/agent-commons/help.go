// SPDX-License-Identifier: MPL-2.0

package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/term"
)

func writeHelp(out io.Writer) error {
	sections := commandSections()
	color := false
	if file, ok := out.(*os.File); ok {
		color = term.IsTerminal(file.Fd()) && os.Getenv("NO_COLOR") == ""
	}
	title := "Agent Commons"
	if color {
		title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#9f4f3d")).Render(title)
	}
	if _, err := fmt.Fprintf(out, "%s\nLocal coordination for project-shaped agent teams.\n\n", title); err != nil {
		return err
	}
	for _, section := range sections {
		header := section.name
		if color {
			header = lipgloss.NewStyle().Bold(true).Render(header)
		}
		if _, err := fmt.Fprintln(out, header); err != nil {
			return err
		}
		for _, row := range section.rows {
			parts := strings.Fields(row)
			if len(parts) > 1 {
				if _, err := fmt.Fprintf(out, "  %-12s %s\n", parts[0], strings.Join(parts[1:], " ")); err != nil {
					return err
				}
			} else if _, err := fmt.Fprintf(out, "  %s\n", strings.TrimSpace(row)); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(out); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(out, "Run 'agent-commons COMMAND --help' for flags and examples.")
	return err
}
