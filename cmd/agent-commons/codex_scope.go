// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"agentcommons/internal/codexlaunch"
)

type codexRoleScope struct {
	Native  codexlaunch.Scope
	Binding string
}

func parseCodexRoleScope(ctx context.Context, command string, args []string, errOut io.Writer) (codexRoleScope, error) {
	f := flag.NewFlagSet(command, flag.ContinueOnError)
	f.SetOutput(errOut)
	config := f.String("config", "", "private enrolled role connection; falls back to AGENT_COMMONS_CONNECTION or an upward .agent-commons/project.json search")
	home := f.String("codex-home", "", "explicit private Codex configuration/session directory")
	if err := f.Parse(args); err != nil {
		return codexRoleScope{}, err
	}
	if !filepath.IsAbs(*home) || f.NArg() != 0 {
		return codexRoleScope{}, fmt.Errorf("%s requires absolute --codex-home", command)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return codexRoleScope{}, err
	}
	resolvedConfig, err := resolveConnectionConfigPath(*config, cwd)
	if err != nil {
		return codexRoleScope{}, err
	}
	return verifyCodexRoleScope(ctx, resolvedConfig, *home)
}

func verifyCodexRoleScope(ctx context.Context, config, home string) (codexRoleScope, error) {
	connection, err := openProjectConnection(config)
	if err != nil {
		return codexRoleScope{}, err
	}
	if err = connection.verifyIdentity(ctx); err != nil {
		return codexRoleScope{}, err
	}
	info, err := os.Lstat(home)
	if err != nil {
		return codexRoleScope{}, err
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return codexRoleScope{}, errors.New("Codex home must be a real private directory")
	}
	nativeHome, err := filepath.EvalSymlinks(home)
	if err != nil {
		return codexRoleScope{}, err
	}
	target, err := filepath.EvalSymlinks(connection.config.Target)
	if err != nil {
		return codexRoleScope{}, err
	}
	socket, err := filepath.EvalSymlinks(connection.config.Socket)
	if err != nil {
		return codexRoleScope{}, err
	}
	path := filepath.Join(connection.config.State, "codex-binding-"+identityDigest(socket+"\x00"+connection.config.Identity))
	return codexRoleScope{
		Native:  codexlaunch.Scope{Identity: connection.config.Identity, Target: target, Home: nativeHome},
		Binding: path,
	}, nil
}
