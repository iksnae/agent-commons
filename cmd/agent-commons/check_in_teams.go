// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"

	"agentcommons/internal/core"
)

type teamDiscoverySnapshot struct {
	Available bool           `json:"available"`
	Page      *core.TeamPage `json:"page,omitempty"`
}

func (connection projectConnection) teamSnapshot(ctx context.Context) (teamDiscoverySnapshot, error) {
	if !connection.supportsTeams {
		return teamDiscoverySnapshot{}, nil
	}
	page, err := rpcCall[core.TeamPage](ctx, connection.client, "teams.list", map[string]int{"limit": 5})
	if err != nil {
		return teamDiscoverySnapshot{}, err
	}
	return teamDiscoverySnapshot{Available: true, Page: &page}, nil
}
