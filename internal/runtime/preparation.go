// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"context"
	"errors"

	"agentcommons/internal/core"
)

func (c CLI) prepareSession(ctx context.Context, session core.Session, delivery core.Delivery) (core.Session, error) {
	if session.Runtime != "codex" || session.RuntimeSessionID != "" || c.PrepareCodex == nil {
		return session, nil
	}
	if c.Checkpoint == nil {
		return session, errors.New("managed preparation requires a durable checkpoint")
	}
	id, err := c.PrepareCodex(ctx, session)
	if err != nil {
		return session, err
	}
	if id == "" {
		return session, errors.New("native preparation returned no identity")
	}
	if err := c.Checkpoint(delivery.ID, id); err != nil {
		return session, err
	}
	session.RuntimeSessionID = id
	return session, nil
}
