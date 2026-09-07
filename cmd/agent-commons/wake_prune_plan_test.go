// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"errors"
	"testing"
)

func TestWakePruningUsesAcknowledgedReceiptsAndPreservesUncertainty(t *testing.T) {
	records := map[string]wakeRecord{"read": {Status: "queued"}, "unread": {Status: "queued"}, "uncertain": {Status: "uncertain"}, "missing": {Status: "queued"}}
	kept, err := planWakePruning(context.Background(), records, "lead", func(cursor string) (wakeReceiptPage, error) {
		if cursor == "" {
			return wakeReceiptPage{Messages: []wakeReceipt{{"read", "lead", true}, {"unread", "lead", false}}, NextCursor: "next"}, nil
		}
		return wakeReceiptPage{Messages: []wakeReceipt{{"uncertain", "lead", true}}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(kept) != 3 || kept["unread"].Status != "queued" || kept["uncertain"].Status != "uncertain" || kept["missing"].Status != "queued" {
		t.Fatal("unsafe prune plan", kept)
	}
	if len(records) != 4 {
		t.Fatal("planning mutated source ledger")
	}
}

func TestWakePruningRejectsIncompleteOrWrongScopeEvidence(t *testing.T) {
	for _, kind := range []string{"rpc error", "cursor cycle", "wrong scope", "canceled"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if kind == "canceled" {
				cancel()
			}
			records := map[string]wakeRecord{"read": {Status: "queued"}}
			kept, err := planWakePruning(ctx, records, "lead", func(cursor string) (wakeReceiptPage, error) {
				switch kind {
				case "rpc error":
					if cursor == "" {
						return wakeReceiptPage{Messages: []wakeReceipt{{"read", "lead", true}}, NextCursor: "next"}, nil
					}
					return wakeReceiptPage{}, errors.New("offline")
				case "wrong scope":
					return wakeReceiptPage{Messages: []wakeReceipt{{"read", "other", true}}}, nil
				default:
					return wakeReceiptPage{Messages: []wakeReceipt{{"read", "lead", true}}, NextCursor: "cycle"}, nil
				}
			})
			if err == nil || kept != nil || len(records) != 1 {
				t.Fatal("incomplete evidence accepted or source changed")
			}
		})
	}
}
