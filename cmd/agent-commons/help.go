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

// Colors are mid-tones so they stay legible against both light and dark
// terminal backgrounds. Weight, hue and dimming separate the four kinds of text
// on this screen: section headers, command names, descriptions, and asides.
var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#9f4f3d"))
	headerStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4f7d9f"))
	nameStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#9f4f3d"))
	asideStyle   = lipgloss.NewStyle().Faint(true)
	commandStyle = lipgloss.NewStyle()
)

// helpIsStyled reports whether the destination is a human's terminal that has
// not asked for plain text. Anything else -- a pipe, a redirect, a captured
// buffer, or NO_COLOR -- gets the unstyled layout.
func helpIsStyled(out io.Writer) bool {
	file, ok := out.(*os.File)
	if !ok {
		return false
	}
	return stylingAllowed(term.IsTerminal(file.Fd()), os.Getenv("NO_COLOR"))
}

// stylingAllowed holds the policy alone, so both of its refusals can be
// asserted without a pseudo-terminal to fake the descriptor half.
func stylingAllowed(isTerminal bool, noColor string) bool {
	return isTerminal && noColor == ""
}

// painter applies a style only when the destination can show one. Plain output
// bypasses lipgloss entirely rather than trusting an empty style to add nothing.
type painter struct{ styled bool }

func (p painter) paint(style lipgloss.Style, text string) string {
	if !p.styled {
		return text
	}
	return style.Render(text)
}

func writeHelp(out io.Writer) error {
	paint := painter{styled: helpIsStyled(out)}
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
