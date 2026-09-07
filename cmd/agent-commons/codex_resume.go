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

type codexResumeReport struct {
	Verified bool   `json:"verified"`
	ThreadID string `json:"threadId"`
	Binding  string `json:"binding"`
}

func runCodexResumeCheckWith(ctx context.Context, args []string, out, errOut io.Writer, start codexStarter) error {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	role, err := parseCodexRoleScope(ctx, "codex-resume-check", args, errOut)
	if err != nil {
		return err
	}
	id, err := codexlaunch.LoadReady(role.Binding, role.Native)
	if err != nil {
		return fmt.Errorf("saved binding is not ready; retain evidence: %w", err)
	}
	bootstrap := &lazyCodexBootstrap{start: start, scope: role.Native}
	resumeErr := codexlaunch.Resume(ctx, bootstrap, role.Native, id)
	err = errors.Join(resumeErr, bootstrap.Close())
	report := codexResumeReport{Verified: err == nil, ThreadID: id, Binding: role.Binding}
	if outputErr := json.NewEncoder(out).Encode(report); outputErr != nil {
		err = errors.Join(err, outputErr)
	}
	if err != nil {
		return fmt.Errorf("Codex resume check failed; binding retained at %q: %w", role.Binding, err)
	}
	return nil
}
