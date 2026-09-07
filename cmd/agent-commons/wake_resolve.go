// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"time"
)

type wakeResolutionReport struct {
	wakeWriteResult
	InspectedStatus string `json:"inspectedStatus"`
	RecordHash      string `json:"recordHash"`
	Decision        string `json:"decision,omitempty"`
}

func runWakeResolution(ctx context.Context, args []string, out, errOut io.Writer) error {
	o, err := parseWakeResolution(args, errOut)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	w, connection, err := openScopedWake(ctx, o.Config, o.Thread)
	if err != nil {
		return err
	}
	defer w.lock.Close()
	record, ok := w.records[o.Message]
	if !ok {
		return errors.New("wake record not found; no record created")
	}
	report := wakeResolutionReport{InspectedStatus: record.Status, RecordHash: wakeRecordHash(record), Decision: o.Decision}
	if o.Decision != "" {
		if err = verifyWakeOperator(ctx, connection.client.socket, o.OperatorToken); err != nil {
			return err
		}
		resolved, resolveErr := resolveWakeRecord(record, o.Expected, o.Decision, o.Evidence)
		if resolveErr != nil {
			return resolveErr
		}
		next := maps.Clone(w.records)
		next[o.Message] = resolved
		err = w.replaceWithBackup(next, &report.wakeWriteResult)
	}
	if outputErr := json.NewEncoder(out).Encode(report); outputErr != nil {
		return errors.Join(err, fmt.Errorf("resolution report failed (backup %q): %w", report.Backup, outputErr))
	}
	if err != nil {
		return fmt.Errorf("resolution incomplete; retain backup %q and inspect ledger before retrying: %w", report.Backup, err)
	}
	return nil
}

func verifyWakeOperator(ctx context.Context, socket, path string) error {
	token, err := readToken(path)
	if err != nil {
		return err
	}
	identity, err := rpcCall[struct {
		Identity string `json:"identity"`
	}](ctx, rpcClient{socket: socket, token: token}, "sessions.capabilities", struct{}{})
	if err != nil {
		return err
	}
	if identity.Identity != "operator" {
		return errors.New("operator credential required for wake resolution")
	}
	return nil
}
