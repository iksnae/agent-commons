// SPDX-License-Identifier: MPL-2.0

package core

import "testing"

func TestRuntimeBindingSurvivesInterruptionBeforeFinish(t *testing.T) {
	s, dir, _ := setup(t)
	d := send(t, s, "lead", "builder", "work")
	if err := s.BindRuntime(d.ID, "prepared-root"); err == nil {
		t.Fatal("bound pending delivery")
	}
	claim(t, s, "builder")
	for i := 0; i < 2; i++ {
		if err := s.BindRuntime(d.ID, "prepared-root"); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.BindRuntime(d.ID, "replacement"); err == nil {
		t.Fatal("replaced prepared root")
	}
	s.Close()
	reopened, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	for _, session := range reopened.Sessions() {
		if session.ID == "builder" && (session.RuntimeSessionID != "prepared-root" || session.Busy) {
			t.Fatal("lost prepared binding after crash")
		}
	}
	if d, err := reopened.Claim("builder"); err != nil || d != nil {
		t.Fatal("interrupted work replayed")
	}
	if err := reopened.BindRuntime(d.ID, "prepared-root"); err == nil {
		t.Fatal("bound interrupted delivery without a new claim")
	}
}
