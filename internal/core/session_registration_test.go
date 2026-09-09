// SPDX-License-Identifier: MPL-2.0

package core

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

// sessionFieldSentinels returns, for every field of Session, a non-zero value a
// client could plausibly send: valid where the field is validated, so that a
// refusal is never an accident of some unrelated check.
//
// The two maps together must cover Session exhaustively. That is the whole
// point: a field added to Session lands in neither map, and the test below
// fails naming it. Session.Retirements was added in v0.0.5 in a conversation
// that did not know about the registration guard, and it reached persisted
// state -- an open ledger entry left the state directory unloadable with no
// supported repair. A list nobody is forced to update is the same defect
// waiting for a fourth field.
func sessionFieldSentinels(t *testing.T) (allow, deny map[string]any) {
	t.Helper()
	// Client-settable. Runtime and Mode belong here because registration
	// validates them as client input; they are the shape of the identity being
	// registered, not state the supervisor owns.
	allow = map[string]any{
		"ID":           "probe-sentinel-id",
		"Target":       t.TempDir(),
		"Name":         "probe-sentinel-name",
		"Role":         "probe-sentinel-role",
		"Team":         "probe-sentinel-team",
		"Policy":       "workflow",
		"Runtime":      "claude",
		"Mode":         "managed",
		"Instructions": "Probe instructions.",
	}
	// Server-owned. Every one of these is written by the service or not at all.
	deny = map[string]any{
		"Attachment":       Attachment{Runtime: "codex", NativeID: "probe-native", LeaseID: "probe-lease", ExpiresAt: 1, Epoch: 1},
		"RuntimeSessionID": "probe-runtime-session",
		"Busy":             true,
		"RetiredAt":        time.Now().UTC().Format(time.RFC3339Nano),
		"RetiredReason":    "Fabricated retirement.",
		// A well-formed CLOSED entry: it passes every load validator, survives
		// restart, and is served back through sessions.list as the service's
		// own evidence. The quiet half of the v0.0.5 defect.
		"Retirements": []Retirement{{
			At: "2026-01-01T00:00:00Z", Reason: "Fabricated.",
			ReinstatedAt: "2026-01-02T00:00:00Z", ReinstatedReason: "Also fabricated.",
		}},
	}
	return allow, deny
}

// The property is: no client-supplied non-zero value on a server-owned Session
// field ever reaches s.data.Sessions.
//
// Asserted at persisted state, not at the error. "The guard returns an error"
// is the adjacent property: it holds while the guards are right AND while a
// field is silently dropped by construction, so it says nothing about what is
// actually at stake, which is what got written down. A refusal and a drop are
// both acceptable outcomes here; persistence is the only failure.
//
// Fields are read back by reflection index rather than by name, so a field
// renamed in Session cannot quietly stop being checked.
func TestRegistrationPersistsOnlyClientSettableSessionFields(t *testing.T) {
	sessionType := reflect.TypeOf(Session{})
	// Both handlers share one branch; the test drives both so a future split
	// cannot leave one of them copying wholesale.
	for _, method := range []string{"sessions.register", "sessions.enroll"} {
		for i := 0; i < sessionType.NumField(); i++ {
			field := sessionType.Field(i)
			t.Run(method+"/"+field.Name, func(t *testing.T) {
				allow, deny := sessionFieldSentinels(t)
				sentinel, settable := allow[field.Name]
				if !settable {
					var known bool
					if sentinel, known = deny[field.Name]; !known {
						t.Fatalf("Session.%s is on neither the client-settable nor the server-owned list in this test. "+
							"Decide which it is: add it to the named-field list in registeredSession and to `allow` here, "+
							"or leave it out of that list and add it to `deny`. Until then nothing checks whether a client can set it.", field.Name)
					}
				}
				value := reflect.ValueOf(sentinel)
				if value.Type() != field.Type {
					t.Fatalf("sentinel for Session.%s is %s, want %s", field.Name, value.Type(), field.Type)
				}
				if value.IsZero() {
					t.Fatalf("sentinel for Session.%s is the zero value, which would prove nothing", field.Name)
				}

				s, _, target := setup(t)
				// The minimum valid identity, plus this one field. Role is
				// non-empty so the enroll candidate scan never adopts one of
				// the fixture's identities instead of minting.
				probe := Session{ID: "probe-identity", Name: "probe-name", Role: "probe-role",
					Target: target, Runtime: "codex", Mode: "manual", Policy: "coordination"}
				reflect.ValueOf(&probe).Elem().Field(i).Set(value)
				raw, err := json.Marshal(probe)
				if err != nil {
					t.Fatal(err)
				}
				_, callErr := s.Call("operator", method, raw)
				stored, persisted := s.data.Sessions[probe.ID]

				if !settable {
					if callErr != nil {
						if persisted {
							t.Fatalf("%s refused a client-supplied Session.%s (%v) and persisted the identity anyway", method, field.Name, callErr)
						}
						return
					}
					if !persisted {
						return
					}
					if got := reflect.ValueOf(stored).Field(i); !got.IsZero() {
						t.Fatalf("Session.%s is server-owned, but a client-supplied %#v reached persisted state as %#v",
							field.Name, sentinel, got.Interface())
					}
					return
				}

				if callErr != nil {
					t.Fatalf("%s refused client-settable Session.%s: %v", method, field.Name, callErr)
				}
				if !persisted {
					t.Fatalf("%s succeeded but persisted no identity under %q", method, probe.ID)
				}
				want := sentinel
				if field.Name == "Target" {
					canonical, err := canonicalTarget(sentinel.(string))
					if err != nil {
						t.Fatal(err)
					}
					want = canonical
				}
				if got := reflect.ValueOf(stored).Field(i).Interface(); !reflect.DeepEqual(got, want) {
					t.Fatalf("client-settable Session.%s round-tripped as %#v, want %#v", field.Name, got, want)
				}
			})
		}
	}
}
