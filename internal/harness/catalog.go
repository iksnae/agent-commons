// SPDX-License-Identifier: MPL-2.0

// Package harness describes implemented Agent Commons integrations, not every
// capability of the upstream agents. Native validation gates remain separate.
package harness

type Capability struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	CheckIn         bool   `json:"checkIn"`
	ManagedDispatch bool   `json:"managedDispatch"`
	ArrivalSignal   bool   `json:"arrivalSignal"`
}

func Catalog() []Capability {
	return []Capability{
		{ID: "claude", Name: "Claude Code", CheckIn: true, ManagedDispatch: true},
		{ID: "codex", Name: "Codex", CheckIn: true, ManagedDispatch: true, ArrivalSignal: true},
		{ID: "pi", Name: "Pi coding agent", CheckIn: true},
		{ID: "hermes", Name: "Hermes Agent", CheckIn: true},
	}
}

func Lookup(id string) (Capability, bool) {
	for _, capability := range Catalog() {
		if capability.ID == id {
			return capability, true
		}
	}
	return Capability{}, false
}

func CanAttach(id string) bool { capability, _ := Lookup(id); return capability.CheckIn }
func CanManage(id string) bool { capability, _ := Lookup(id); return capability.ManagedDispatch }
