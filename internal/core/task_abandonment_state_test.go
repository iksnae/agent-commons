// SPDX-License-Identifier: MPL-2.0

package core

import (
	"strings"
	"testing"
)

// An abandoned task on a schema that predates the feature is a hand edit, and
// the binary that wrote that schema tests only `Status == "accepted"` for
// terminality -- so it would let the task be resubmitted and accepted. This is
// the schema-floor half of the load guard, which the team, team-work and
// retirement guards all have.
func TestLoadRefusesAbandonedTasksBelowSchemaFour(t *testing.T) {
	s, dir, task := abandonFixture(t)
	if s.data.SchemaVersion >= abandonmentSchema {
		t.Fatalf("fixture is already at schema %d; this test needs an older one", s.data.SchemaVersion)
	}
	message := reopenWithCorruptedState(t, s, dir, func(s *Service) {
		v := s.data.Tasks[task.ID]
		v.Status = "abandoned"
		v.AbandonEvidence = "Backdated onto a schema that never knew about abandonment."
		s.data.Tasks[v.ID] = v
	})
	if message == "" {
		t.Fatal("an abandoned task loaded on a pre-abandonment schema")
	}
	if !strings.Contains(message, "schema 4") {
		t.Fatalf("refusal does not name the schema floor it enforces: %q", message)
	}
}

// The per-record half. Each case names the exact message it expects: several
// invariants would refuse a badly enough broken task, and "loading failed"
// cannot tell them apart. Every case starts from a real abandonment, so the
// schema floor is already satisfied and cannot be what refused.
func TestLoadRefusesIncompleteAbandonmentRecords(t *testing.T) {
	for _, c := range []struct {
		name    string
		corrupt func(Task) Task
		want    string
	}{
		{"blank evidence", func(v Task) Task {
			v.AbandonEvidence = "   "
			return v
		}, "abandoned task is missing its evidence"},
		{"absent evidence", func(v Task) Task {
			v.AbandonEvidence = ""
			return v
		}, "abandoned task is missing its evidence"},
		{"evidence past the bound", func(v Task) Task {
			v.AbandonEvidence = strings.Repeat("e", 8193)
			return v
		}, "abandoned task evidence exceeds the 8KiB bound"},
		{"evidence on a task that is not abandoned", func(v Task) Task {
			v.Status = "submitted"
			return v
		}, "abandonment evidence recorded on a task that is not abandoned"},
	} {
		t.Run(c.name, func(t *testing.T) {
			s, dir, task := abandonFixture(t)
			rpc(t, s, "operator", "tasks.abandon", abandonArgs(task.ID))
			if s.data.SchemaVersion != abandonmentSchema {
				t.Fatalf("fixture did not reach the abandonment schema: %d", s.data.SchemaVersion)
			}
			message := reopenWithCorruptedState(t, s, dir, func(s *Service) {
				s.data.Tasks[task.ID] = c.corrupt(s.data.Tasks[task.ID])
			})
			if message == "" {
				t.Fatal("an incomplete abandonment record loaded")
			}
			if !strings.Contains(message, c.want) {
				t.Fatalf("refusal came from a different invariant:\n got %q\nwant %q", message, c.want)
			}
		})
	}
}

// The guard must not refuse the state abandon itself writes, or every case
// above would pass on a validator that rejects everything.
func TestLoadAcceptsTheStateAbandonWrites(t *testing.T) {
	s, dir, task := abandonFixture(t)
	rpc(t, s, "operator", "tasks.abandon", abandonArgs(task.ID))
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(dir)
	if err != nil {
		t.Fatalf("the guard refused state written by tasks.abandon: %v", err)
	}
	defer reopened.Close()
	if reopened.data.Tasks[task.ID].Status != "abandoned" {
		t.Fatalf("reopened task is %q, want abandoned", reopened.data.Tasks[task.ID].Status)
	}
}
