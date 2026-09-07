// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"errors"
	"maps"
)

type wakeReceipt struct {
	ID           string `json:"id"`
	To           string `json:"to"`
	Acknowledged bool   `json:"acknowledged"`
}

type wakeReceiptPage struct {
	Messages   []wakeReceipt `json:"messages"`
	NextCursor string        `json:"nextCursor"`
}

func planWakePruning(ctx context.Context, records map[string]wakeRecord, identity string, fetch func(string) (wakeReceiptPage, error)) (map[string]wakeRecord, error) {
	kept := maps.Clone(records)
	seen := map[string]bool{}
	cursor := ""
	for page := 0; page < 1000; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if seen[cursor] {
			return nil, errors.New("inbox cursor cycle; ledger unchanged")
		}
		seen[cursor] = true
		result, err := fetch(cursor)
		if err != nil {
			return nil, err
		}
		for _, receipt := range result.Messages {
			if receipt.To != identity {
				return nil, errors.New("inbox receipt identity mismatch")
			}
			if receipt.Acknowledged && kept[receipt.ID].Status == "queued" {
				delete(kept, receipt.ID)
			}
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if result.NextCursor == "" {
			return kept, nil
		}
		cursor = result.NextCursor
	}
	return nil, errors.New("inbox exceeds maintenance page budget; ledger unchanged")
}
