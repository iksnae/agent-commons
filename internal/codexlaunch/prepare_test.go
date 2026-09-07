// SPDX-License-Identifier: MPL-2.0

package codexlaunch

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type fakeJournal struct {
	stages []string
	fail   string
}

func (j *fakeJournal) Reserve(Scope) error  { return j.record("reserved") }
func (j *fakeJournal) Created(string) error { return j.record("created") }
func (j *fakeJournal) Ready(string) error   { return j.record("ready") }
func (j *fakeJournal) record(stage string) error {
	if stage == j.fail {
		return errors.New("storage failure")
	}
	j.stages = append(j.stages, stage)
	return nil
}

type fakeRPC struct {
	calls        []string
	fail         string
	journal      *fakeJournal
	response     json.RawMessage
	readResponse json.RawMessage
}

func (r *fakeRPC) Call(_ context.Context, method string, _ any) (json.RawMessage, error) {
	if method == "thread/start" && len(r.journal.stages) != 1 {
		return nil, errors.New("no reservation before create")
	}
	if method == "thread/inject_items" && len(r.journal.stages) != 2 {
		return nil, errors.New("no identity saved before history")
	}
	r.calls = append(r.calls, method)
	if method == r.fail {
		return nil, errors.New("RPC uncertain")
	}
	if method == "thread/read" && r.readResponse != nil {
		return r.readResponse, nil
	}
	return r.response, nil
}

func TestPreparationDoesNotMarkChangedReadbackReady(t *testing.T) {
	journal := &fakeJournal{}
	root := json.RawMessage(`{"thread":{"id":"00000000-0000-0000-0000-000000000001","cwd":"/project","forkedFromId":null,"parentThreadId":null}}`)
	rpc := &fakeRPC{journal: journal, response: root, readResponse: json.RawMessage(strings.ReplaceAll(string(root), "000000000001", "000000000002"))}
	id, err := Prepare(context.Background(), rpc, journal, Scope{Identity: "lead", Target: "/project", Home: "/private/codex"})
	if err == nil || id != "00000000-0000-0000-0000-000000000001" || len(journal.stages) != 2 {
		t.Fatal("changed readback marked ready or lost known identity", err)
	}
}

func TestPreparationCheckpointsBeforeExternalEffects(t *testing.T) {
	for _, failure := range []string{"", "reserved", "thread/start", "created", "thread/inject_items", "thread/read", "ready"} {
		t.Run(failure, func(t *testing.T) {
			journal := &fakeJournal{fail: failure}
			rpc := &fakeRPC{journal: journal, fail: failure, response: json.RawMessage(`{"thread":{"id":"00000000-0000-0000-0000-000000000001","cwd":"/project","forkedFromId":null,"parentThreadId":null}}`)}
			id, err := Prepare(context.Background(), rpc, journal, Scope{Identity: "lead", Target: "/project", Home: "/private/codex"})
			if failure == "" {
				if err != nil || id == "" || len(journal.stages) != 3 {
					t.Fatal("preparation incomplete", err)
				}
			} else if err == nil {
				t.Fatal("failure hidden")
			}
			if failure == "reserved" && len(rpc.calls) != 0 {
				t.Fatal("failed reservation created thread")
			}
			if failure == "created" && len(rpc.calls) != 1 {
				t.Fatal("failed identity save continued")
			}
		})
	}
}

func TestPreparationRejectsWrongTargetForkAndIncompleteMetadata(t *testing.T) {
	for _, thread := range []string{
		`{"id":"root","cwd":"/elsewhere","forkedFromId":null,"parentThreadId":null}`,
		`{"id":"root","cwd":"/project","forkedFromId":"parent","parentThreadId":null}`,
		`{"id":"root","cwd":"/project","forkedFromId":null,"parentThreadId":"parent"}`,
		`{"id":"root","cwd":"/project"}`,
	} {
		journal := &fakeJournal{}
		thread = strings.ReplaceAll(thread, `"root"`, `"00000000-0000-0000-0000-000000000001"`)
		rpc := &fakeRPC{journal: journal, response: json.RawMessage(`{"thread":` + thread + `}`)}
		if _, err := Prepare(context.Background(), rpc, journal, Scope{Identity: "lead", Target: "/project", Home: "/private/codex"}); err == nil || len(journal.stages) != 1 || len(rpc.calls) != 1 {
			t.Fatal("unverified identity accepted", err)
		}
	}
}
