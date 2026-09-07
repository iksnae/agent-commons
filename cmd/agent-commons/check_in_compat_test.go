// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"agentcommons/internal/core"
)

func TestCheckInNegotiatesLegacyAndAdvertisedBoardOrdering(t *testing.T) {
	for _, advertised := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy", true: "advertised-failure"}[advertised], func(t *testing.T) {
			connection := boardCompatibilityConnection(t, advertised)
			// Verification must clear stale capability flags, not carry them across services.
			connection.supportsTeams, connection.supportsBoardOrder = true, true
			if err := connection.verifyIdentity(context.Background()); err != nil {
				t.Fatal(err)
			}
			if connection.supportsTeams || connection.supportsBoardOrder != advertised {
				t.Fatal("capabilities not refreshed")
			}
			snapshot, err := connection.snapshot(context.Background(), core.Attachment{})
			if advertised {
				if err == nil {
					t.Fatal("advertised failure silently fell back")
				}
			} else if err != nil || snapshot.Teams.Available {
				t.Fatal("legacy service compatibility failed", err)
			}
		})
	}
}

func boardCompatibilityConnection(t *testing.T, advertised bool) projectConnection {
	t.Helper()
	dir, err := os.MkdirTemp("", "ac-compat-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	socket := filepath.Join(dir, "rpc.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	peer := core.Session{ID: "reader", Target: "/project", Name: "Reader", Role: "learner"}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Method string
			Params map[string]any
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		var result any
		switch request.Method {
		case "sessions.list":
			result = []core.Session{peer}
		case "sessions.capabilities":
			result = map[string]any{"identity": peer.ID}
			if advertised {
				result.(map[string]any)["boardOrderingAvailable"] = true
			}
		case "inbox.page":
			result = map[string]any{"messages": []any{}}
		case "board.list":
			order, supplied := request.Params["order"]
			if supplied != advertised || supplied && order != "newest" {
				t.Error("incorrect negotiated board order", request.Params)
			}
			if advertised {
				json.NewEncoder(w).Encode(map[string]string{"error": "board unavailable"})
				return
			}
			result = map[string]any{"posts": []any{}, "nextCursor": ""}
		default:
			t.Error("unexpected legacy service call", request.Method)
			w.WriteHeader(400)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"result": result})
	}))
	server.Listener.Close()
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)
	return projectConnection{config: connectionConfig{Identity: peer.ID, Target: peer.Target, Name: peer.Name, Role: peer.Role}, client: rpcClient{socket: socket, token: "fixture"}}
}
