// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"agentcommons/internal/codexlaunch"
	"agentcommons/internal/core"
)

// Preparation is not wired into serve: its starter must first prove the managed
// runner's configuration isolation. The standalone starter loads native config.
func prepareManagedCodex(ctx context.Context, state string, session core.Session, start codexStarter) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	home := os.Getenv("CODEX_HOME")
	if home == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		home = filepath.Join(userHome, ".codex")
	}
	info, err := os.Lstat(home)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(home) || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return "", errors.New("managed Codex requires an existing private runtime home")
	}
	home, err = filepath.EvalSymlinks(home)
	if err != nil {
		return "", err
	}
	scope := codexlaunch.Scope{Identity: session.ID, Target: session.Target, Home: home}
	path := filepath.Join(state, "managed-codex-"+identityDigest(session.ID))
	bootstrap := &lazyCodexBootstrap{start: start, scope: scope}
	var id string
	if _, statErr := os.Lstat(path); errors.Is(statErr, os.ErrNotExist) {
		id, err = codexlaunch.Prepare(ctx, bootstrap, codexlaunch.NewDirectoryJournal(path), scope)
	} else if statErr != nil {
		err = statErr
	} else {
		// A completed preparation may outlive a failed service checkpoint.
		// Partial journals are refused before touching the native runtime.
		id, err = codexlaunch.LoadReady(path, scope)
		if err == nil {
			err = codexlaunch.Resume(ctx, bootstrap, scope, id)
		}
	}
	err = errors.Join(err, bootstrap.Close())
	if err != nil {
		return "", err
	}
	return id, nil
}
