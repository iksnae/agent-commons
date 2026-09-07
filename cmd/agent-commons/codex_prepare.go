// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"agentcommons/internal/codexlaunch"
)

type codexBootstrap interface {
	codexlaunch.Caller
	io.Closer
}
type codexStarter func(context.Context, codexlaunch.Scope) (codexBootstrap, error)
type codexPreparationReport struct {
	Prepared bool   `json:"prepared"`
	ThreadID string `json:"threadId,omitempty"`
	Binding  string `json:"binding"`
}

func runCodexPrepareWith(ctx context.Context, args []string, out, errOut io.Writer, start codexStarter) error {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	role, err := parseCodexRoleScope(ctx, "codex-prepare", args, errOut)
	if err != nil {
		return err
	}
	scope, path := role.Native, role.Binding
	bootstrap := &lazyCodexBootstrap{start: start, scope: scope}
	journal := codexlaunch.NewDirectoryJournal(path)
	id, prepareErr := codexlaunch.Prepare(ctx, bootstrap, journal, scope)
	err = errors.Join(prepareErr, bootstrap.Close(), journal.Close())
	report := codexPreparationReport{Prepared: err == nil, ThreadID: id, Binding: path}
	if outputErr := json.NewEncoder(out).Encode(report); outputErr != nil {
		err = errors.Join(err, outputErr)
	}
	if err != nil {
		return fmt.Errorf("Codex preparation incomplete; retain and inspect binding %q before another attempt: %w", path, err)
	}
	return nil
}

type lazyCodexBootstrap struct {
	start   codexStarter
	scope   codexlaunch.Scope
	process codexBootstrap
}

func (b *lazyCodexBootstrap) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if b.process == nil {
		var err error
		b.process, err = b.start(ctx, b.scope)
		if err != nil {
			return nil, err
		}
	}
	return b.process.Call(ctx, method, params)
}
func (b *lazyCodexBootstrap) Close() error {
	if b.process != nil {
		return b.process.Close()
	}
	return nil
}
