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

// This file holds the whole human presentation vocabulary for the CLI: the
// palette, the styling policy, and the block a command writes when it is
// talking to a person rather than to a parser. Every human rendering goes
// through it, so a second, drifting copy of the colour choices cannot appear.
// Nothing here knows what any command does.

// jsonFlagUsage is the one sentence every command carrying --json shows, so the
// escape from the human default reads identically wherever it is registered.
// The flag is always per-command, on that command's own FlagSet: there is no
// process-wide output mode, and a command whose stdout is a machine contract
// (check-in) deliberately does not have it at all.
const jsonFlagUsage = "write the machine-readable JSON report instead of the human summary"

// Colors are mid-tones so they stay legible against both light and dark
// terminal backgrounds. Weight, hue and dimming separate the four kinds of text
// on these screens: section headers, command names and labels, descriptions,
// and asides.
var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#9f4f3d"))
	headerStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4f7d9f"))
	nameStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#9f4f3d"))
	asideStyle   = lipgloss.NewStyle().Faint(true)
	commandStyle = lipgloss.NewStyle()
)

// writerIsStyled reports whether the destination is a human's terminal that has
// not asked for plain text. Anything else -- a pipe, a redirect, a captured
// buffer, or NO_COLOR -- gets the unstyled layout.
func writerIsStyled(out io.Writer) bool {
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

// reportLine is one rendered line. An empty label means the text is already
// complete -- a headline, an aside, or a blank separator -- and is written as
// it stands. A label makes it a field, and fields are aligned against each
// other when the block is written.
type reportLine struct{ label, text string }

// report accumulates a human-facing block and writes it in one pass, so the
// label column can be measured across every field the command actually added
// rather than guessed at a constant.
type report struct {
	paint painter
	out   io.Writer
	lines []reportLine
}

func newReport(out io.Writer) *report {
	return &report{paint: painter{styled: writerIsStyled(out)}, out: out}
}

// headline states what happened, in one line, at the top of a block.
func (r *report) headline(text string) {
	r.lines = append(r.lines, reportLine{text: r.paint.paint(headerStyle, text)})
}

// field records one labelled value. Values are written verbatim: they are
// paths, identities and runtime names, and a styled path is a path a reader
// cannot copy cleanly out of a terminal.
func (r *report) field(label, value string) {
	r.lines = append(r.lines, reportLine{label: label, text: value})
}

// note is a dimmed line: a caveat, a limit, or the command to run next.
func (r *report) note(text string) {
	r.lines = append(r.lines, reportLine{text: r.paint.paint(asideStyle, text)})
}

// line writes text with no styling and no label, for a renderer that has
// already painted its own spans.
func (r *report) line(text string) {
	r.lines = append(r.lines, reportLine{text: text})
}

// blank separates two blocks.
func (r *report) blank() {
	r.lines = append(r.lines, reportLine{})
}

// write emits the whole block. Padding is measured on the unstyled label,
// because a styled one carries escape sequences that no column count should
// include.
func (r *report) write() error {
	width := 0
	for _, line := range r.lines {
		if len(line.label) > width {
			width = len(line.label)
		}
	}
	var screen strings.Builder
	for _, line := range r.lines {
		if line.label == "" {
			screen.WriteString(line.text + "\n")
			continue
		}
		fmt.Fprintf(&screen, "  %s%s  %s\n",
			r.paint.paint(nameStyle, line.label),
			strings.Repeat(" ", width-len(line.label)),
			line.text)
	}
	_, err := io.WriteString(r.out, screen.String())
	return err
}
