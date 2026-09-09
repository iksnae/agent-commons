// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"agentcommons/internal/console"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"
)

func runConsole(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer) error {
	flags := flag.NewFlagSet("console", flag.ContinueOnError)
	flags.SetOutput(errOut)
	config := flags.String("config", "", "explicit private role connection file; falls back to AGENT_COMMONS_CONNECTION or an upward .agent-commons/project.json search; never falls back to operator")
	once := flags.Bool("once", false, "print one JSON snapshot; automatic when input or output is not a terminal")
	if err := parseFlags(flags, args, out); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("console takes no positional arguments")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	resolvedConfig, err := resolveConnectionConfigPath(*config, cwd)
	if err != nil {
		return err
	}
	connection, err := openProjectConnection(resolvedConfig)
	if err != nil {
		return err
	}
	if *once || !consoleTerminal(in) || !consoleTerminal(out) {
		document := func(ctx context.Context) (consoleSnapshotJSON, error) {
			return loadConsoleSnapshotJSON(ctx, connection)
		}
		return writeConsoleSnapshot(ctx, document, out)
	}
	load := func(ctx context.Context) (console.Snapshot, error) { return consoleSnapshot(ctx, connection) }
	_, err = tea.NewProgram(console.New(ctx, load), tea.WithContext(ctx), tea.WithInput(in), tea.WithOutput(out)).Run()
	return err
}

func consoleTerminal(stream any) bool {
	file, ok := stream.(interface{ Fd() uintptr })
	return ok && term.IsTerminal(file.Fd())
}

// consoleSnapshot feeds the terminal UI. At this step it projects the CLI
// contract document, because the UI happens to display exactly the rows the
// document carries. That is a coincidence of the current design, not a rule:
// when the UI grows its own typed rows this stops being a projection and builds
// them itself. The dependency points this way — UI derived from the document,
// never the document derived from the UI — so that a change on the UI side is
// an edit here rather than a silent change to what the CLI prints.
func consoleSnapshot(ctx context.Context, connection projectConnection) (console.Snapshot, error) {
	document, err := loadConsoleSnapshotJSON(ctx, connection)
	return console.Snapshot{Scope: document.Scope, Agents: document.Agents, Tasks: document.Tasks}, err
}
