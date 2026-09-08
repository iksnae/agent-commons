// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMatchLaunchTargetAcceptsTheRootAndWhatIsBeneathIt fixes the containment
// boundary. A launch from a subdirectory of an enrolled project is the normal
// case and must work, but "beneath" is a trust boundary: a sibling that merely
// shares a textual prefix, a parent, an unrelated tree, or a path that climbs
// out with ".." must all still be refused.
func TestMatchLaunchTargetAcceptsTheRootAndWhatIsBeneathIt(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "proj")
	sibling := filepath.Join(parent, "proj-other")
	nested := filepath.Join(root, "internal", "deep")
	for _, dir := range []string{nested, sibling} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	// A symlink whose spelling differs from the real path on both sides, the
	// same shape as macOS resolving /var to /private/var.
	link := filepath.Join(parent, "link-to-proj")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}

	accepted := map[string]string{
		"the enrolled root itself":          root,
		"a direct subdirectory":             filepath.Join(root, "internal"),
		"a deeply nested subdirectory":      nested,
		"a symlinked spelling":              link,
		"a subdirectory reached by symlink": filepath.Join(link, "internal"),
		"an uncleaned path inside the root": filepath.Join(root, "internal", "..", "internal", "deep"),
	}
	for name, directory := range accepted {
		t.Run("accepts "+name, func(t *testing.T) {
			if err := matchLaunchTarget(directory, root); err != nil {
				t.Fatalf("rejected a launch inside the enrolled project: %v", err)
			}
		})
	}

	refused := map[string]string{
		"a sibling sharing a textual prefix":  sibling,
		"the parent of the enrolled root":     parent,
		"an unrelated tree":                   t.TempDir(),
		"a path climbing out of the root":     filepath.Join(root, "internal", "..", ".."),
		"a sibling reached through a symlink": filepath.Join(link, "..", "proj-other"),
	}
	for name, directory := range refused {
		t.Run("refuses "+name, func(t *testing.T) {
			if err := matchLaunchTarget(directory, root); err == nil {
				t.Fatal("accepted a launch outside the enrolled project")
			}
		})
	}

	t.Run("refuses a directory that does not exist", func(t *testing.T) {
		if err := matchLaunchTarget(filepath.Join(root, "absent"), root); err == nil {
			t.Fatal("accepted a launch directory that cannot be resolved")
		}
	})

	t.Run("matches when the target is the symlinked spelling", func(t *testing.T) {
		if err := matchLaunchTarget(nested, link); err != nil {
			t.Fatalf("symlinked enrolled target rejected its own subdirectory: %v", err)
		}
	})
}

// TestLaunchContextChecksInFromASubdirectory proves the boundary through the
// command itself, not only its helper.
func TestLaunchContextChecksInFromASubdirectory(t *testing.T) {
	state := onboardingService(t)
	parent := t.TempDir()
	target := filepath.Join(parent, "proj")
	sub := filepath.Join(target, "cmd", "tool")
	sibling := filepath.Join(parent, "proj-other")
	for _, dir := range []string{sub, sibling} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	enrolled := onboardingCommand(t, "enroll", "--state", state, "--target", target, "--name", "lead", "--role", "lead")
	var enrollment struct {
		Config string `json:"config"`
	}
	if err := json.Unmarshal(enrolled, &enrollment); err != nil {
		t.Fatal(err)
	}

	launch := func(directory, session string) error {
		input, _ := json.Marshal(launchEvent{Event: "SessionStart", Session: session, Directory: directory, Source: "startup"})
		return runLaunchContext(context.Background(), []string{"--config", enrollment.Config, "--claude-version", "2.1.236"}, bytes.NewReader(input), io.Discard, io.Discard)
	}
	if err := launch(sub, "nested-launch"); err != nil {
		t.Fatalf("a launch from a subdirectory of the enrolled project failed: %v", err)
	}
	if err := launch(sibling, "sibling-launch"); err == nil {
		t.Fatal("a launch from a prefix-sharing sibling was accepted")
	}
}

// TestLaunchContextIsQuietWithoutAProjectAndLoudWhenOneIsConfigured fixes the
// hook's exit-code contract: no project at all is not an error, but a project
// that is initialized and unenrolled is.
func TestLaunchContextIsQuietWithoutAProjectAndLoudWhenOneIsConfigured(t *testing.T) {
	t.Setenv("AGENT_COMMONS_CONNECTION", "")

	t.Run("silent outside any Agent Commons project", func(t *testing.T) {
		input, _ := json.Marshal(launchEvent{Event: "SessionStart", Session: "unrelated", Directory: t.TempDir(), Source: "startup"})
		var out, errOut bytes.Buffer
		if err := runLaunchContext(context.Background(), []string{"--claude-version", "2.1.236"}, bytes.NewReader(input), &out, &errOut); err != nil {
			t.Fatalf("a launch in an unrelated repository reported an error: %v", err)
		}
		if out.Len() != 0 || errOut.Len() != 0 {
			t.Fatalf("a launch in an unrelated repository was not silent: %q %q", out.String(), errOut.String())
		}
	})

	t.Run("reports an initialized project with no enrolled role", func(t *testing.T) {
		root := t.TempDir()
		writeProjectManifest(t, root, projectDefaults{Version: 1, ProjectRoot: root, Name: "lead", Role: "lead", State: t.TempDir()})
		input, _ := json.Marshal(launchEvent{Event: "SessionStart", Session: "unenrolled", Directory: root, Source: "startup"})
		var out bytes.Buffer
		err := runLaunchContext(context.Background(), []string{"--claude-version", "2.1.236"}, bytes.NewReader(input), &out, io.Discard)
		if err == nil {
			t.Fatal("an initialized but unenrolled project was silently ignored")
		}
		if !strings.Contains(err.Error(), "agent-commons enroll") {
			t.Fatalf("the remedy was not named: %v", err)
		}
	})

	t.Run("reports a broken explicit connection", func(t *testing.T) {
		input, _ := json.Marshal(launchEvent{Event: "SessionStart", Session: "broken", Directory: t.TempDir(), Source: "startup"})
		err := runLaunchContext(context.Background(), []string{"--config", filepath.Join(t.TempDir(), "absent.json"), "--claude-version", "2.1.236"}, bytes.NewReader(input), io.Discard, io.Discard)
		if err == nil {
			t.Fatal("an explicit connection that cannot be opened was silently ignored")
		}
	})
}
