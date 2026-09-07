// SPDX-License-Identifier: MPL-2.0

package codexlaunch

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type resumeRPC struct {
	calls         []string
	params        []any
	read, resumed json.RawMessage
	fail          string
	afterRead     context.CancelFunc
}

var errResumeRPC = errors.New("native resume uncertain")

func (r *resumeRPC) Call(_ context.Context, method string, params any) (json.RawMessage, error) {
	r.calls = append(r.calls, method)
	r.params = append(r.params, params)
	if method == r.fail {
		return nil, errResumeRPC
	}
	if method == "thread/read" {
		if r.afterRead != nil {
			r.afterRead()
		}
		return r.read, nil
	}
	return r.resumed, nil
}

func TestResumeCancellationPreventsFurtherNativeCalls(t *testing.T) {
	for _, beforeRead := range []bool{true, false} {
		rpc, scope := resumeFixture()
		ctx, cancel := context.WithCancel(context.Background())
		wantCalls := 1
		if beforeRead {
			cancel()
			wantCalls = 0
		} else {
			rpc.afterRead = cancel
		}
		err := Resume(ctx, rpc, scope, savedThread)
		cancel()
		if !errors.Is(err, context.Canceled) || len(rpc.calls) != wantCalls {
			t.Fatal("canceled verification continued native work", err, rpc.calls)
		}
	}
}

func resumeFixture() (*resumeRPC, Scope) {
	root := json.RawMessage(`{"thread":{"id":"` + savedThread + `","cwd":"/project","forkedFromId":null,"parentThreadId":null}}`)
	return &resumeRPC{read: root, resumed: root}, Scope{Identity: "lead", Target: "/project", Home: "/private/codex"}
}

func TestResumeVerifiesRootBeforeReadOnlyResume(t *testing.T) {
	rpc, scope := resumeFixture()
	if err := Resume(context.Background(), rpc, scope, savedThread); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rpc.calls, []string{"thread/read", "thread/resume"}) {
		t.Fatal("unexpected native effects", rpc.calls)
	}
	want := []any{
		map[string]any{"threadId": savedThread, "includeTurns": false},
		map[string]any{"threadId": savedThread, "cwd": scope.Target, "approvalPolicy": "never", "sandbox": "read-only"},
	}
	if !reflect.DeepEqual(rpc.params, want) {
		t.Fatal("resume did not preserve exact identity and restrictions", rpc.params)
	}
}

func TestResumeRejectsChangedMetadataBeforeAndAfterResume(t *testing.T) {
	for _, change := range []struct{ from, to string }{
		{savedThread, "00000000-0000-0000-0000-000000000002"},
		{"/project", "/elsewhere"},
		{`"forkedFromId":null`, `"forkedFromId":"parent"`},
		{`"parentThreadId":null`, `"parentThreadId":"parent"`},
		{`,"parentThreadId":null`, ``},
	} {
		for _, phase := range []string{"read", "resume"} {
			t.Run(phase+change.from, func(t *testing.T) {
				rpc, scope := resumeFixture()
				changed := json.RawMessage(strings.Replace(string(rpc.read), change.from, change.to, 1))
				wantCalls := 1
				if phase == "read" {
					rpc.read = changed
				} else {
					rpc.resumed = changed
					wantCalls = 2
				}
				if err := Resume(context.Background(), rpc, scope, savedThread); err == nil || len(rpc.calls) != wantCalls {
					t.Fatal("changed identity accepted or execution continued", err, rpc.calls)
				}
			})
		}
	}
}

func TestResumeDoesNotRetryNativeFailures(t *testing.T) {
	for index, method := range []string{"thread/read", "thread/resume"} {
		rpc, scope := resumeFixture()
		rpc.fail = method
		if err := Resume(context.Background(), rpc, scope, savedThread); !errors.Is(err, errResumeRPC) || len(rpc.calls) != index+1 {
			t.Fatal("native failure hidden or retried", err, rpc.calls)
		}
	}
}

func TestResumeRejectsInvalidInputBeforeNativeCalls(t *testing.T) {
	rpc, scope := resumeFixture()
	if err := Resume(context.Background(), rpc, scope, "latest"); err == nil {
		t.Fatal("session alias accepted")
	}
	scope.Identity = ""
	if err := Resume(context.Background(), rpc, scope, savedThread); err == nil {
		t.Fatal("missing scope accepted")
	}
	if len(rpc.calls) != 0 {
		t.Fatal("invalid input reached native runtime")
	}
}
