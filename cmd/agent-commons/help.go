// SPDX-License-Identifier: MPL-2.0

package main

import (
	"fmt"
	"io"
	"strings"
)

// The palette, the styling policy and the painter this screen uses live in
// presentation.go, shared with every other human rendering in the CLI.

// usageSynopsis states the shape of an invocation without naming a command.
// commandSections is the only place commands are listed, and a placeholder
// restates none of them, so this line cannot fall out of step with the catalog.
const usageSynopsis = "agent-commons <command> [flags]"

// startHere is hand-written prose for a first-time operator, deliberately not
// generated from the catalog: it names an order of two commands, which the
// catalog does not record and should not have to.
const startHere = "New here? Run 'init' to set this project up, then 'doctor' to check it."

// connectSection is the block the getting-started line belongs beside.
const connectSection = "Connect a session"

// commandColumn is the width of the command name column, including the space
// before a description.
const commandColumn = 13

func writeHelp(out io.Writer) error {
	paint := painter{styled: writerIsStyled(out)}
	var screen strings.Builder

	fmt.Fprintf(&screen, "%s %s\n", paint.paint(titleStyle, "Agent Commons"), paint.paint(asideStyle, version))
	fmt.Fprintf(&screen, "%s\n\n", paint.paint(asideStyle, "Local coordination for project-shaped agent teams."))
	fmt.Fprintf(&screen, "%s %s\n\n", paint.paint(asideStyle, "Usage:"), usageSynopsis)

	for _, section := range commandSections() {
		fmt.Fprintf(&screen, "%s\n", paint.paint(headerStyle, section.name))
		for _, row := range section.rows {
			writeCommandRow(&screen, paint, row)
		}
		if section.name == connectSection {
			fmt.Fprintf(&screen, "  %s\n", paint.paint(asideStyle, startHere))
		}
		screen.WriteString("\n")
	}

	fmt.Fprintf(&screen, "%s\n", paint.paint(asideStyle, "Run 'agent-commons COMMAND --help' for flags and examples."))

	_, err := io.WriteString(out, screen.String())
	return err
}

// writeCommandRow renders one catalog row as a name and a description. Padding
// is measured on the unstyled name, because a styled one carries escape
// sequences that no column count should include.
func writeCommandRow(screen *strings.Builder, paint painter, row string) {
	fields := strings.Fields(row)
	if len(fields) == 0 {
		return
	}
	name := fields[0]
	if len(fields) == 1 {
		fmt.Fprintf(screen, "  %s\n", paint.paint(nameStyle, name))
		return
	}
	gap := commandColumn - len(name)
	if gap < 1 {
		gap = 1
	}
	fmt.Fprintf(screen, "  %s%s%s\n",
		paint.paint(nameStyle, name),
		strings.Repeat(" ", gap),
		paint.paint(commandStyle, strings.Join(fields[1:], " ")))
}
