// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"agentcommons/internal/console"
	"agentcommons/internal/core"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"
)

func runConsole(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer) error {
	flags := flag.NewFlagSet("console", flag.ContinueOnError)
	flags.SetOutput(errOut)
	config := flags.String("config", "", "explicit private role connection file; falls back to AGENT_COMMONS_CONNECTION or an upward .agent-commons/project.json search; never falls back to operator")
	once := flags.Bool("once", false, "print one JSON snapshot; automatic when input or output is not a terminal")
	if err := flags.Parse(args); err != nil {
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
	load := func(ctx context.Context) (console.Snapshot, error) { return consoleSnapshot(ctx, connection) }
	if *once || !consoleTerminal(in) || !consoleTerminal(out) {
		return writeConsoleSnapshot(ctx, load, out)
	}
	_, err = tea.NewProgram(console.New(ctx, load), tea.WithContext(ctx), tea.WithInput(in), tea.WithOutput(out)).Run()
	return err
}

func consoleTerminal(stream any) bool {
	file, ok := stream.(interface{ Fd() uintptr })
	return ok && term.IsTerminal(file.Fd())
}

func writeConsoleSnapshot(ctx context.Context, load console.Loader, out io.Writer) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	snapshot, err := load(ctx)
	if err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(snapshot)
}

func consoleSnapshot(ctx context.Context, connection projectConnection) (console.Snapshot, error) {
	snapshot := console.Snapshot{Scope: connection.config.Target}
	if err := connection.verifyIdentity(ctx); err != nil {
		return snapshot, err
	}
	peers, err := rpcCall[[]core.Session](ctx, connection.client, "sessions.list", struct{}{})
	if err != nil {
		return snapshot, err
	}
	for _, peer := range peers {
		state := "not attached"
		if peer.Busy {
			state = "managed delivery running"
		} else if peer.Attachment.ExpiresAt > time.Now().Unix() {
			state = "attachment lease current (reachability unverified)"
		}
		snapshot.Agents = append(snapshot.Agents, fmt.Sprintf("%s / %s | %s | %s | %s", peer.Name, peer.Role, peer.Runtime, peer.ID, state))
	}
	tasks, err := rpcCall[[]core.Task](ctx, connection.client, "tasks.list", struct{}{})
	if err != nil {
		return snapshot, err
	}
	for _, task := range tasks {
		snapshot.Tasks = append(snapshot.Tasks, fmt.Sprintf("%s | %s | revision %d | %s", task.ID, task.Status, task.Revision, task.Title))
	}
	sort.Strings(snapshot.Agents)
	sort.Strings(snapshot.Tasks)
	return snapshot, nil
}
