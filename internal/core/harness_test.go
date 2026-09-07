// SPDX-License-Identifier: MPL-2.0

package core

import "testing"

func TestPiAndHermesParticipateInScopedCoordination(t *testing.T) {
	for _, runtime := range []string{"pi", "hermes"} {
		t.Run(runtime, func(t *testing.T) {
			s, _, target := setup(t)
			rpc(t, s, "operator", "sessions.register", Session{ID: "harness-lead", Target: target, Runtime: runtime, Mode: "manual", Name: "harness-lead", Role: "lead"})
			rpc(t, s, "operator", "sessions.register", Session{ID: "harness-peer", Target: target, Runtime: "manual", Mode: "manual"})
			a := rpc(t, s, "harness-lead", "sessions.attach", map[string]string{"nativeId": "native-" + runtime, "runtime": runtime, "target": target}).(Attachment)
			if a.Runtime != runtime {
				t.Fatal("runtime identity lost")
			}
			rpc(t, s, "harness-peer", "messages.send", map[string]string{"to": "harness-lead", "text": "Review project learnings", "idempotencyKey": "one"})
			rpc(t, s, "harness-lead", "inbox.page", map[string]any{})
			denied(t, s, "harness-lead", "sessions.attach", map[string]string{"nativeId": "other", "runtime": runtime, "target": target})
			rpc(t, s, "harness-lead", "sessions.detach", map[string]string{"nativeId": a.NativeID, "leaseId": a.LeaseID})
			denied(t, s, "operator", "sessions.register", Session{ID: "unverified-managed", Target: target, Runtime: runtime, Mode: "managed", Policy: "workflow"})
		})
	}
}
