// SPDX-License-Identifier: MPL-2.0

package codexlaunch

import (
	"context"
	"errors"
	"fmt"
)

// Resume requires a verified scope, an ID from LoadReady, and an exclusively
// owned native connection using that scope's Codex home. It verifies root
// metadata before and after resume. It never attaches a role, starts a model
// turn, repairs a binding, or retries an uncertain native operation.
func Resume(ctx context.Context, rpc Caller, scope Scope, id string) error {
	if err := scope.validate(); err != nil {
		return err
	}
	if !threadID.MatchString(id) {
		return errors.New("resume requires an exact saved thread ID")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	response, err := rpc.Call(ctx, "thread/read", map[string]any{"threadId": id, "includeTurns": false})
	if err != nil {
		return fmt.Errorf("inspect saved native root: %w", err)
	}
	if _, err = rootIdentity(response, scope.Target, id); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	response, err = rpc.Call(ctx, "thread/resume", map[string]any{
		"threadId": id, "cwd": scope.Target, "approvalPolicy": "never", "sandbox": "read-only",
	})
	if err != nil {
		return fmt.Errorf("resume saved native root: %w", err)
	}
	_, err = rootIdentity(response, scope.Target, id)
	return err
}
