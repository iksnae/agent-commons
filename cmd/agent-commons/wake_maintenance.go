// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"time"
)

type wakeMaintenanceReport struct {
	Eligible  int    `json:"eligible"`
	Remaining int    `json:"remaining"`
	Applied   bool   `json:"applied"`
	Uncertain bool   `json:"uncertain"`
	Backup    string `json:"backup,omitempty"`
}

func runWakeMaintenance(ctx context.Context, args []string, out, errOut io.Writer) error {
	flags := flag.NewFlagSet("wake-maintain", flag.ContinueOnError)
	flags.SetOutput(errOut)
	config := flags.String("config", "", "private enrolled-role connection")
	thread := flags.String("codex-thread", "", "exact thread UUID whose wake history is maintained")
	apply := flags.Bool("apply", false, "apply pruning after saving a private backup; default previews")
	risk := flags.Bool("acknowledge-replay-risk", false, "acknowledge that restoring older inbox state can repeat pruned notifications")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *config == "" || *thread == "" || flags.NArg() != 0 || *apply != *risk {
		return errors.New("wake-maintain requires --config FILE --codex-thread UUID; applying requires both --apply and --acknowledge-replay-risk")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	connection, err := openProjectConnection(*config)
	if err != nil {
		return err
	}
	if err = connection.verifyIdentity(ctx); err != nil {
		return err
	}
	socket, err := filepath.EvalSymlinks(connection.config.Socket)
	if err != nil {
		return err
	}
	w, err := openWake(ctx, connection.config.State, *thread, socket+"\x00"+connection.config.Identity, io.Discard)
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
		err = w.prune(kept, &report)
	}
	if outputErr := json.NewEncoder(out).Encode(report); outputErr != nil {
		return errors.Join(err, fmt.Errorf("maintenance report failed (backup %q): %w", report.Backup, outputErr))
	}
	if err != nil {
		return fmt.Errorf("maintenance incomplete; retain backup %q and inspect ledger before retrying: %w", report.Backup, err)
	}
	return nil
}
