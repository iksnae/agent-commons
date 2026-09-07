// SPDX-License-Identifier: MPL-2.0

package supervision

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type recordedCommands struct {
	calls [][]string
	fail  bool
}

func (r *recordedCommands) Run(_ context.Context, name string, args ...string) (string, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	if r.fail {
		return "", errors.New("unavailable")
	}
	return "observed", nil
}

func TestControlScopesJobsAndStopsOnFailure(t *testing.T) {
	label := "io.agent-commons.0123456789abcdef"
	for _, platform := range []string{"darwin", "linux"} {
		t.Run(platform, func(t *testing.T) {
			runner := &recordedCommands{}
			ext := map[string]string{"darwin": ".plist", "linux": ".service"}[platform]
			c := Control{Platform: platform, UID: 501, File: "/private/jobs/" + label + ext, Label: label, Commands: runner}
			out, err := c.Apply(context.Background(), "status")
			if err != nil || out != "observed" || len(runner.calls) != 1 {
				t.Fatal(out, err, runner.calls)
			}
			expected := []string{"/bin/launchctl", "print", "gui/501/" + label}
			if platform == "linux" {
				expected = []string{"/usr/bin/systemctl", "--user", "show", "--property=LoadState,ActiveState,SubState,MainPID", label + ext}
			}
			if !reflect.DeepEqual(runner.calls[0], expected) {
				t.Fatal(runner.calls)
			}
			runner.calls = nil
			runner.fail = true
			if _, err = c.Apply(context.Background(), "enable"); err == nil || len(runner.calls) != 1 {
				t.Fatal("continued after failure", err, runner.calls)
			}
		})
	}
}

func TestControlRejectsInvalidIdentityWithoutCommands(t *testing.T) {
	runner := &recordedCommands{}
	c := Control{Platform: "linux", UID: 501, File: "/jobs/other.service", Label: "other", Commands: runner}
	if _, err := c.Apply(context.Background(), "stop"); err == nil || len(runner.calls) != 0 {
		t.Fatal("acted on foreign job")
	}
}
