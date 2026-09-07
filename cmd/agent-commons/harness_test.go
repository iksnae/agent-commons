// SPDX-License-Identifier: MPL-2.0

package main

import (
	"encoding/json"
	"testing"
)

func TestPiAndHermesCheckInUsesExplicitNativeIdentity(t *testing.T) {
	for _, runtime := range []string{"pi", "hermes"} {
		t.Run(runtime, func(t *testing.T) {
			state := onboardingService(t)
			var enrollment struct {
				Config string `json:"config"`
			}
			if err := json.Unmarshal(onboardingCommand(t, "enroll", "--state", state, "--target", t.TempDir(), "--name", "lead", "--role", "lead"), &enrollment); err != nil {
				t.Fatal(err)
			}
			if _, err := resolveCheckIn(onboardingOptions{Config: enrollment.Config, Runtime: runtime}); err == nil {
				t.Fatal("guessed native identity")
			}
			var snapshot checkInSnapshot
			if err := json.Unmarshal(onboardingCommand(t, "check-in", "--config", enrollment.Config, "--runtime", runtime, "--native-session", "exact-native-id"), &snapshot); err != nil {
				t.Fatal(err)
			}
			if snapshot.Attachment.Runtime != runtime || snapshot.Attachment.NativeID != "exact-native-id" {
				t.Fatal("incorrect attachment binding")
			}
		})
	}
}
