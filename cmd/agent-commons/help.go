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

type helpSection struct {
	name string
	rows []string
}

func writeHelp(out io.Writer) error {
	sections := []helpSection{
		{name: "Connect a session", rows: []string{
			"init         bootstrap this project and enroll the first role",
			"enroll       create a project + agent role connection",
			"check-in     read onboarding, inbox and project context",
			"doctor       diagnose a connection without changing state",
		}},
		{name: "Run the local service", rows: []string{
			"serve        run the local coordination service",
			"service      install or control the per-user service",
			"console      open the read-only command center",
		}},
		{name: "Harness integration", rows: []string{
			"connect-mcp  serve the scoped MCP connection",
			"harnesses    show runtime capabilities",
			"discover     inspect Claude or Codex project definitions",
			"inventory    list discovered project resources",
		}},
		{name: "Inspect and maintain", rows: []string{
			"bundle       install, verify or remove a local bundle",
			"watch        print unread inbox notifications as JSON",
			"call         invoke one RPC method",
			"methods      print the agent-facing RPC catalog",
		}},
	}

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
