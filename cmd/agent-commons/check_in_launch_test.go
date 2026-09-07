// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"context"
	"testing"
)

func TestCheckInLaunchDirectoryMustMatchEnrolledProject(t *testing.T) {
	args, connection := codexPrepareFixture(t)
	for _, directory := range []string{t.TempDir(), "relative-project"} {
		var output bytes.Buffer
		err := runOnboarding(context.Background(), []string{"check-in", "--config", args[1], "--runtime", "pi", "--native-session", "wrong-launch", "--launch-directory", directory}, &output, &output)
		if err == nil {
			t.Fatal("accepted wrong launch directory")
		}
	}
	var output bytes.Buffer
	if err := runOnboarding(context.Background(), []string{"check-in", "--config", args[1], "--runtime", "pi", "--native-session", "correct-launch", "--launch-directory", connection.config.Target}, &output, &output); err != nil {
		t.Fatal("matching launch failed or rejected launch occupied role", err)
	}
}
