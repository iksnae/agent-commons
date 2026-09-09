// SPDX-License-Identifier: MPL-2.0

package core

import (
	"errors"
	"fmt"
	"strings"
)

// abandonmentSchema is the durable schema number task abandonment takes. Teams
// take 2, team-scoped work 3, and session retirement 5.
const abandonmentSchema = 4

// enableTaskAbandonment advances the state schema the first time a task is
// actually abandoned, taking a pre-migration snapshot first. It is lazy on
// purpose: a directory that never abandons anything keeps the older schema and
// stays readable by an older binary.
//
// The bump is what makes a downgrade safe. An older binary tests only
// `Status == "accepted"` where this one tests terminality, so it would let an
// abandoned task be resubmitted and accepted. That binary already refuses a
// schema it does not know, so raising the number arms a refusal it ships with.
// Callers must run every validation before this: a refused abandonment must
// write no backup and advance no number.
func (s *Service) enableTaskAbandonment() error {
	if s.data.SchemaVersion >= abandonmentSchema {
		return nil
	}
	if err := s.backupSchema(fmt.Sprintf("pre-task-abandonment-schema-%d-", s.data.SchemaVersion)); err != nil {
		return err
	}
	s.data.SchemaVersion = abandonmentSchema
	return nil
}

// validateTaskAbandonment refuses to load state whose abandonment records
// disagree with the schema that carries them or with each other. It mirrors
// validateRetirement in both halves: a schema floor for the feature's records,
// then the per-record invariant.
//
// The floor is the load-time half of what enableTaskAbandonment arms at write
// time. A binary older than schema 4 tests only `Status == "accepted"` for
// terminality, so an abandoned task carried on schema 3 or below is exactly the
// state that binary would reopen and let be resubmitted. The pre-migration
// snapshots this package retains (schema_backup.go) are the files an operator
// hand-edits during a repair, which is how such a record comes to be written.
//
// The per-record half exists because Status and AbandonEvidence are two halves
// of one record that abandon always writes together, so a saved task carrying
// one without the other did not come from abandon.
func (s *Service) validateTaskAbandonment() error {
	for _, t := range s.data.Tasks {
		abandoned := t.Status == "abandoned"
		if !abandoned && t.AbandonEvidence == "" {
			continue
		}
		if s.data.SchemaVersion < abandonmentSchema {
			return errors.New("abandoned task records require schema 4")
		}
		if !abandoned {
			return errors.New("abandonment evidence recorded on a task that is not abandoned")
		}
		if strings.TrimSpace(t.AbandonEvidence) == "" {
			return errors.New("abandoned task is missing its evidence")
		}
		if len(t.AbandonEvidence) > 8192 {
			return errors.New("abandoned task evidence exceeds the 8KiB bound")
		}
	}
	return nil
}

// terminalTaskStatus reports whether a task has reached a status no workflow
// step can move. Acceptance and abandonment are both terminal and they stay
// distinct: abandonment records that the work stopped, never that it passed.
// The acceptance predicate is deliberately not expressed through this helper.
func terminalTaskStatus(status string) bool {
	return status == "accepted" || status == "abandoned"
}

// abandon retires a task the workflow can no longer move. A submitted result
// carrying a rejection at the current revision only advances when its author
// resubmits, so a task whose author will never act again pins every one of its
// participants in an unresolved obligation. Abandonment is the operator's
// escape from that, not a way to pass the task: it never approves anything,
// never enqueues work, and cannot be reversed.
//
// The recorded output and every review verdict are retained exactly as they
// stand, because they are the evidence of why the work stopped.
func (s *Service) abandon(actor string, p params) (any, error) {
	if actor != "operator" {
		return nil, errors.New("operator required")
	}
	if strings.TrimSpace(p.Evidence) == "" || len(p.Evidence) > 8192 {
		return nil, errors.New("abandonment evidence (max 8KiB) required")
	}
	t, ok := s.data.Tasks[p.ID]
	if !ok {
		return nil, errors.New("task unavailable")
	}
	if terminalTaskStatus(t.Status) {
		return nil, errors.New("terminal task cannot be abandoned")
	}
	if err := s.enableTaskAbandonment(); err != nil {
		return nil, err
	}
	t.Status = "abandoned"
	t.AbandonEvidence = p.Evidence
	s.data.Tasks[t.ID] = t
	return cloneTask(t), nil
}
