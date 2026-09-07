// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"agentcommons/internal/codexlaunch"
	"agentcommons/internal/core"
)

type managedBootstrap struct {
	fakeBootstrap
	methods *[]string
	fail    string
}

func (b *managedBootstrap) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	*b.methods = append(*b.methods, method)
	if method == b.fail {
		return nil, errors.New("injected native failure")
	}
	return b.fakeBootstrap.Call(ctx, method, params)
}

func TestManagedCodexReusesReadyPreparationButRefusesPartialJournal(t *testing.T) {
	for _, fail := range []string{"", "thread/inject_items"} {
		t.Run("fail-"+fail, func(t *testing.T) {
			home := t.TempDir()
			if err := os.Chmod(home, 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("CODEX_HOME", home)
			state := t.TempDir()
			if err := os.Chmod(state, 0700); err != nil {
				t.Fatal(err)
			}
			session := core.Session{ID: "role", Runtime: "codex", Mode: "managed", Target: t.TempDir()}
			methods := []string{}
			processes := []*managedBootstrap{}
			start := func(_ context.Context, scope codexlaunch.Scope) (codexBootstrap, error) {
				b := &managedBootstrap{fakeBootstrap: fakeBootstrap{scope: scope}, methods: &methods, fail: fail}
				processes = append(processes, b)
				return b, nil
			}
			id, err := prepareManagedCodex(context.Background(), state, session, start)
			if len(processes) != 1 {
				t.Fatal("native preparation did not start", err)
			}
			if (err != nil) != (fail != "") || (fail == "" && id != testThread) {
				t.Fatal("wrong preparation outcome", err)
			}
			before := len(methods)
			id, err = prepareManagedCodex(context.Background(), state, session, start)
			if fail != "" {
				if err == nil || id != "" || len(methods) != before {
					t.Fatal("replayed partial preparation")
				}
			} else {
				if err != nil || id != testThread {
					t.Fatal("ready root could not be recovered", err)
				}
				for _, method := range methods[before:] {
					if method == "thread/start" || method == "thread/inject_items" {
						t.Fatal("created replacement native root")
					}
				}
			}
			for _, p := range processes {
				if !p.closed {
					t.Fatal("preparation process not closed")
				}
			}
		})
	}
}
