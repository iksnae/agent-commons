// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"agentcommons/internal/codexlaunch"
	"agentcommons/internal/core"
)

type closeCheckedBootstrap struct {
	managedBootstrap
	check func()
}

func (b *closeCheckedBootstrap) Close() error {
	b.check()
	return b.managedBootstrap.Close()
}

func TestManagedPreparationHoldsBindingThroughNativeShutdown(t *testing.T) {
	for _, failure := range []string{"", "thread/inject_items"} {
		t.Run("failure-"+failure, func(t *testing.T) {
			home, state := t.TempDir(), t.TempDir()
			for _, path := range []string{home, state} {
				if err := os.Chmod(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("CODEX_HOME", home)
			session := core.Session{ID: "role", Target: t.TempDir(), Runtime: "codex", Mode: "managed"}
			path := filepath.Join(state, "managed-codex-"+identityDigest(session.ID))
			var scope codexlaunch.Scope
			closed := 0
			start := func(_ context.Context, native codexlaunch.Scope) (codexBootstrap, error) {
				scope = native
				methods := []string{}
				return &closeCheckedBootstrap{
					managedBootstrap: managedBootstrap{fakeBootstrap: fakeBootstrap{scope: native}, methods: &methods, fail: failure},
					check: func() {
						closed++
						lease, err := codexlaunch.AcquireReady(path, native)
						if lease != nil {
							_ = lease.Close()
						}
						if !errors.Is(err, codexlaunch.ErrBindingBusy) {
							t.Error("binding released before native shutdown", err)
						}
					},
				}, nil
			}
			for attempt := 0; attempt < 2; attempt++ {
				_, err := prepareManagedCodex(context.Background(), state, session, start)
				if (err != nil) != (failure != "") {
					t.Fatal("unexpected preparation result", err)
				}
				lease, err := codexlaunch.AcquireReady(path, scope)
				if lease != nil {
					_ = lease.Close()
				}
				if errors.Is(err, codexlaunch.ErrBindingBusy) || (failure == "" && err != nil) {
					t.Fatal("shutdown retained lease", err)
				}
			}
			want := 2
			if failure != "" {
				want = 1
			}
			if closed != want {
				t.Fatal("unexpected native launches", closed)
			}
		})
	}
}
