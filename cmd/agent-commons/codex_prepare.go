// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	f := flag.NewFlagSet("codex-prepare", flag.ContinueOnError)
	f.SetOutput(errOut)
	config := f.String("config", "", "private enrolled role connection")
	home := f.String("codex-home", "", "explicit private Codex configuration/session directory")
	if err := f.Parse(args); err != nil {
		return err
	}
	if *config == "" || !filepath.IsAbs(*home) || f.NArg() != 0 {
		return errors.New("codex-prepare requires --config and absolute --codex-home")
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	connection, err := openProjectConnection(*config)
	if err != nil {
		return err
	}
	if err = connection.verifyIdentity(ctx); err != nil {
		return err
	}
	info, err := os.Lstat(*home)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return errors.New("Codex home must be a real private directory")
	}
	nativeHome, err := filepath.EvalSymlinks(*home)
	if err != nil {
		return err
	}
	target, err := filepath.EvalSymlinks(connection.config.Target)
	if err != nil {
		return err
	}
	socket, err := filepath.EvalSymlinks(connection.config.Socket)
	if err != nil {
		return err
	}
	path := filepath.Join(connection.config.State, "codex-binding-"+identityDigest(socket+"\x00"+connection.config.Identity))
	scope := codexlaunch.Scope{Identity: connection.config.Identity, Target: target, Home: nativeHome}
	bootstrap := &lazyCodexBootstrap{start: start, scope: scope}
	id, prepareErr := codexlaunch.Prepare(ctx, bootstrap, codexlaunch.NewDirectoryJournal(path), scope)
	err = errors.Join(prepareErr, bootstrap.Close())
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
