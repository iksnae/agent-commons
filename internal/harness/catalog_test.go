// SPDX-License-Identifier: MPL-2.0

package harness

import "testing"

func TestFoundationalHarnessesDistinguishCoordinationFromDispatch(t *testing.T) {
	if len(Catalog()) != 4 {
		t.Fatal("expected Claude, Codex, Pi and Hermes")
	}
	for _, name := range []string{"claude", "codex", "pi", "hermes"} {
		if !CanAttach(name) {
			t.Fatal("missing coordination support", name)
		}
	}
	for _, name := range []string{"pi", "hermes", "manual", "unknown"} {
		if CanManage(name) {
			t.Fatal("unimplemented managed adapter advertised", name)
		}
	}
	list := Catalog()
	list[0].ID = "mutated"
	if !CanAttach("claude") || CanAttach("mutated") {
		t.Fatal("catalog mutation escaped caller")
	}
}
