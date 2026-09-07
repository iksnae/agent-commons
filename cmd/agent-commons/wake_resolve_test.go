// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestWakeResolveRequiresOperatorAndPreservesAuditEvidence(t *testing.T) {
	for _, decision := range []string{"retry", "suppress"} {
		t.Run(decision, func(t *testing.T) {
			args, connection, path := wakeMaintenanceFixture(t)
			args[0] = "wake-resolve"
			args = append(args, "--message-id", "uncertain")
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var preview bytes.Buffer
			if err = run(context.Background(), args, nil, &preview, io.Discard); err != nil {
				t.Fatal(err)
			}
			var report wakeResolutionReport
			if err = json.Unmarshal(preview.Bytes(), &report); err != nil || report.Applied || report.RecordHash == "" || report.InspectedStatus != "uncertain" {
				t.Fatal("bad inspection", err)
			}
			inbox, err := rpcCall[json.RawMessage](context.Background(), connection.client, "inbox.page", struct{}{})
			if err != nil {
				t.Fatal(err)
			}
			apply := append(append([]string{}, args...), "--expected-hash", report.RecordHash, "--decision", decision, "--evidence", "Operator reviewed history.", "--acknowledge-notification-risk", "--operator-token-file")
			if err = run(context.Background(), append(append([]string{}, apply...), connection.config.TokenFile), nil, io.Discard, io.Discard); err == nil {
				t.Fatal("role credential authorized resolution")
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("inspection/refusal changed ledger", err)
			}
			apply = append(apply, filepath.Join(connection.config.State, "operator.token"))
			var output bytes.Buffer
			if err = run(context.Background(), apply, nil, &output, io.Discard); err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(output.Bytes(), &report); err != nil || !report.Applied || report.Uncertain {
				t.Fatal("resolution not durable", err)
			}
			backup, err := os.ReadFile(report.Backup)
			if err != nil || !bytes.Equal(before, backup) {
				t.Fatal("prior attempt not backed up", err)
			}
			var records map[string]wakeRecord
			if err = privateReadLimit(path, &records, maxWakeLedger); err != nil {
				t.Fatal(err)
			}
			if records["uncertain"].Resolution == nil || records["uncertain"].Resolution.Decision != decision {
				t.Fatal("operator evidence not recorded")
			}
			if err = run(context.Background(), apply, nil, io.Discard, io.Discard); err == nil {
				t.Fatal("stale decision reused")
			}
			current, err := rpcCall[json.RawMessage](context.Background(), connection.client, "inbox.page", struct{}{})
			if err != nil || !bytes.Equal(inbox, current) {
				t.Fatal("resolution changed inbox", err)
			}
		})
	}
}
