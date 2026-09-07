// SPDX-License-Identifier: MPL-2.0

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type wakeResolution struct {
	At           string `json:"at"`
	Decision     string `json:"decision"`
	Evidence     string `json:"evidence"`
	PreviousHash string `json:"previousHash"`
}

func wakeRecordHash(record wakeRecord) string {
	// This fixed struct contains only JSON-serializable strings and structs.
	data, _ := json.Marshal(record)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func resolveWakeRecord(record wakeRecord, expected, decision, evidence string) (wakeRecord, error) {
	if expected != wakeRecordHash(record) {
		return record, errors.New("wake record changed; inspect it again before deciding")
	}
	if record.Status != "uncertain" && record.Status != "attempting" {
		return record, errors.New("only uncertain or attempting wakes can be resolved")
	}
	if strings.TrimSpace(evidence) == "" || len(evidence) > 4096 {
		return record, errors.New("operator evidence required, at most 4096 bytes")
	}
	switch decision {
	case "retry":
		record.Status = "retry-approved"
	case "suppress":
		record.Status = "suppressed"
	default:
		return record, errors.New("decision must be retry or suppress")
	}
	record.At = time.Now().UTC().Format(time.RFC3339Nano)
	record.Resolution = &wakeResolution{At: record.At, Decision: decision, Evidence: evidence, PreviousHash: expected}
	return record, nil
}

func (w *wakeWriter) markWakeRecords(ids []string, status string) {
	for _, id := range ids {
		record := w.records[id]
		record.Status = status
		record.At = time.Now().UTC().Format(time.RFC3339Nano)
		w.records[id] = record
	}
}
