// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"agentcommons/internal/core"
)

func TestCheckInShowsInvitationWithoutJoiningOrAcknowledging(t *testing.T) {
	for _, runtime := range []string{"hermes", "pi", "claude", "codex"} {
		t.Run(runtime, func(t *testing.T) {
			args, connection := codexPrepareFixture(t)
			token, err := readToken(filepath.Join(filepath.Dir(connection.config.Socket), "operator.token"))
			if err != nil {
				t.Fatal(err)
			}
			operator := rpcClient{socket: connection.config.Socket, token: token}
			_, err = rpcCall[core.TeamView](context.Background(), operator, "teams.create", map[string]any{"id": "product", "target": connection.config.Target, "title": "Product", "text": "Brief"})
			if err != nil {
				t.Fatal(err)
			}
			_, err = rpcCall[json.RawMessage](context.Background(), operator, "teams.invite", map[string]any{"id": "product", "target": connection.config.Target, "to": connection.config.Identity})
			if err != nil {
				t.Fatal(err)
			}
			output := onboardingCommand(t, "check-in", "--config", args[1], "--runtime", runtime, "--native-session", "exact-fixture-id")
			var snapshot checkInSnapshot
			if err = json.Unmarshal(output, &snapshot); err != nil {
				t.Fatal(err)
			}
			if !snapshot.Teams.Available || snapshot.Teams.Page == nil || len(snapshot.Teams.Page.Teams) != 1 || snapshot.Teams.Page.Teams[0].Status != "invited" {
				t.Fatal("missing invitation snapshot", snapshot.Teams)
			}
			view, err := rpcCall[core.TeamView](context.Background(), connection.client, "teams.get", map[string]any{"id": "product"})
			if err != nil || view.Status != "invited" || len(view.Members) != 0 {
				t.Fatal("check-in joined team", err)
			}
			var inbox struct {
				Messages []core.Delivery `json:"messages"`
			}
			if err = json.Unmarshal(snapshot.Inbox, &inbox); err != nil {
				t.Fatal(err)
			}
			if len(inbox.Messages) != 3 {
				t.Fatal("expected welcome and invitation messages", len(inbox.Messages))
			}
			for _, message := range inbox.Messages {
				if message.Acknowledged {
					t.Fatal("check-in acknowledged a message")
				}
			}
		})
	}
}

func TestTeamSnapshotReportsUnsupportedWithoutCallingLegacyService(t *testing.T) {
	snapshot, err := (projectConnection{}).teamSnapshot(context.Background())
	if err != nil || snapshot.Available || snapshot.Page != nil {
		t.Fatal("unsupported service misreported", snapshot, err)
	}
}

func TestTeamSnapshotDoesNotHideAdvertisedServiceFailure(t *testing.T) {
	connection := projectConnection{supportsTeams: true}
	if _, err := connection.teamSnapshot(context.Background()); err == nil {
		t.Fatal("failed advertised discovery reported as unsupported")
	}
}
