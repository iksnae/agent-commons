// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"agentcommons/internal/supervision"
)

type serviceCommands struct {
	fail  bool
	calls [][]string
}

func (s *serviceCommands) Run(_ context.Context, name string, args ...string) (string, error) {
	s.calls = append(s.calls, append([]string{name}, args...))
	if s.fail {
		return "", errors.New("supervisor unavailable")
	}
	return "", nil
}

func TestServiceRemovalRetainsConfigurationAndState(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "supervisor failure"}[fail], func(t *testing.T) {
			state := t.TempDir()
			marker := filepath.Join(state, "preserve")
			if err := os.WriteFile(marker, []byte("inbox"), 0600); err != nil {
				t.Fatal(err)
			}
			file, err := supervision.InstallFile(t.TempDir(), supervision.Options{Platform: runtime.GOOS, Binary: "/opt/agent-commons", State: state, SearchPath: "/usr/bin"})
			if err != nil {
				t.Fatal(err)
			}
			commands := &serviceCommands{fail: fail}
			var output bytes.Buffer
			err = controlService(context.Background(), "remove", file, &output, commands)
			if fail {
				if err == nil {
					t.Fatal("ignored supervisor failure")
				}
				if _, err = supervision.ReadInstalled(file); err != nil {
					t.Fatal("moved files after failed disable", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				var result map[string]string
				if err = json.Unmarshal(output.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if _, err = supervision.ReadInstalled(filepath.Join(result["retained"], filepath.Base(file))); err != nil {
					t.Fatal("lost recoverable config", err)
				}
				if runtime.GOOS == "linux" && len(commands.calls) != 3 {
					t.Fatal("missing post-removal reload", commands.calls)
				}
			}
			data, err := os.ReadFile(marker)
			if err != nil || string(data) != "inbox" {
				t.Fatal("state changed", err)
			}
		})
	}
}
