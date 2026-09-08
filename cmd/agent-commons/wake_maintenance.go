// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"
)

type wakeMaintenanceReport struct {
	wakeWriteResult
	Eligible  int `json:"eligible"`
	Remaining int `json:"remaining"`
}

func runWakeMaintenance(ctx context.Context, args []string, out, errOut io.Writer) error {
	flags := flag.NewFlagSet("wake-maintain", flag.ContinueOnError)
	flags.SetOutput(errOut)
	config := flags.String("config", "", "private enrolled-role connection; falls back to AGENT_COMMONS_CONNECTION or an upward .agent-commons/project.json search")
	thread := flags.String("codex-thread", "", "exact thread UUID whose wake history is maintained")
	apply := flags.Bool("apply", false, "apply pruning after saving a private backup; default previews")
	risk := flags.Bool("acknowledge-replay-risk", false, "acknowledge that restoring older inbox state can repeat pruned notifications")
	if err := parseFlags(flags, args); err != nil {
		return err
	}
	if *thread == "" || flags.NArg() != 0 || *apply != *risk {
		return errors.New("wake-maintain requires --codex-thread UUID; applying requires both --apply and --acknowledge-replay-risk")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	resolvedConfig, err := resolveConnectionConfigPath(*config, cwd)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	w, connection, err := openScopedWake(ctx, resolvedConfig, *thread)
	if err != nil {
		return err
	}
	defer w.lock.Close()
	kept, err := planWakePruning(ctx, w.records, connection.config.Identity, func(cursor string) (wakeReceiptPage, error) {
		return rpcCall[wakeReceiptPage](ctx, connection.client, "inbox.page", map[string]any{"cursor": cursor, "limit": 100})
	})
	if err != nil {
		return err
	}
	report := wakeMaintenanceReport{Eligible: len(w.records) - len(kept), Remaining: len(kept)}
	if *apply && report.Eligible > 0 {
		err = w.replaceWithBackup(kept, &report.wakeWriteResult)
	}
	if outputErr := json.NewEncoder(out).Encode(report); outputErr != nil {
		return errors.Join(err, fmt.Errorf("maintenance report failed (backup %q): %w", report.Backup, outputErr))
	}
	if err != nil {
		return fmt.Errorf("maintenance incomplete; retain backup %q and inspect ledger before retrying: %w", report.Backup, err)
	}
	return nil
}
