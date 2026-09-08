// SPDX-License-Identifier: MPL-2.0

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeProjectManifest writes a private .agent-commons/project.json under
// root, matching exactly what runInit writes (init.go), and returns the
// manifest path.
func writeProjectManifest(t *testing.T, root string, manifest projectDefaults) string {
	t.Helper()
	projectDir := filepath.Join(root, ".agent-commons")
	if err := os.MkdirAll(projectDir, 0700); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(projectDir, "project.json")
	if err := os.WriteFile(path, append(data, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestResolveConnectionConfigPathExplicitBeatsEnvAndManifest(t *testing.T) {
	t.Setenv("AGENT_COMMONS_CONNECTION", "/env/should-not-be-used.json")
	root := t.TempDir()
	state := t.TempDir()
	writeProjectManifest(t, root, projectDefaults{Version: 1, ProjectRoot: root, Name: "lead", Role: "lead", State: state})
	got, err := resolveConnectionConfigPath("/explicit/connection.json", root)
	if err != nil {
		t.Fatal(err)
	}
	if got != "/explicit/connection.json" {
		t.Fatalf("explicit --config was overridden: got %q", got)
	}
}

func TestResolveConnectionConfigPathEnvBeatsManifest(t *testing.T) {
	root := t.TempDir()
	state := t.TempDir()
	writeProjectManifest(t, root, projectDefaults{Version: 1, ProjectRoot: root, Name: "lead", Role: "lead", State: state})
	t.Setenv("AGENT_COMMONS_CONNECTION", "/env/connection.json")
	got, err := resolveConnectionConfigPath("", root)
	if err != nil {
		t.Fatal(err)
	}
	if got != "/env/connection.json" {
		t.Fatalf("environment connection was overridden by manifest: got %q", got)
	}
}

// enrolledManifest builds a project root + state dir with a manifest and a
// present (dummy but well-formed) connection file at the exact path
// prepareEnrollment would derive for the same target/name/role, so tests
// assert against the real derivation rather than a copy of it.
func enrolledManifest(t *testing.T, name, role string) (root, state, connectionPath string) {
	t.Helper()
	t.Setenv("AGENT_COMMONS_CONNECTION", "")
	root = t.TempDir()
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	state = t.TempDir()
	if err := os.Chmod(state, 0700); err != nil {
		t.Fatal(err)
	}
	writeProjectManifest(t, root, projectDefaults{Version: 1, ProjectRoot: root, Name: name, Role: role, State: state})
	connection, err := prepareEnrollment(onboardingOptions{Name: name, Role: role, Target: root, State: state})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(connection.Path, []byte(`{"version":1,"identity":"agent-x","target":"`+root+`","name":"`+name+`","role":"`+role+`","socket":"","tokenFile":"","state":"`+state+`"}`+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return root, state, connection.Path
}

func TestResolveConnectionConfigPathManifestFromProjectRoot(t *testing.T) {
	root, _, connectionPath := enrolledManifest(t, "lead", "lead")
	got, err := resolveConnectionConfigPath("", root)
	if err != nil {
		t.Fatal(err)
	}
	if got != connectionPath {
		t.Fatalf("resolved %q, want %q", got, connectionPath)
	}
}

func TestResolveConnectionConfigPathManifestFromNestedSubdirectory(t *testing.T) {
	root, _, connectionPath := enrolledManifest(t, "lead", "lead")
	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	got, err := resolveConnectionConfigPath("", nested)
	if err != nil {
		t.Fatal(err)
	}
	if got != connectionPath {
		t.Fatalf("resolved %q from nested dir, want %q", got, connectionPath)
	}
}

func TestResolveConnectionConfigPathMatchesRealEnrollDerivation(t *testing.T) {
	root, state, connectionPath := enrolledManifest(t, "reviewer", "workspace-lead")
	connection, err := prepareEnrollment(onboardingOptions{Name: "reviewer", Role: "workspace-lead", Target: root, State: state})
	if err != nil {
		t.Fatal(err)
	}
	if connection.Path != connectionPath {
		t.Fatalf("test fixture derivation %q disagrees with prepareEnrollment %q", connectionPath, connection.Path)
	}
	got, err := resolveConnectionConfigPath("", root)
	if err != nil {
		t.Fatal(err)
	}
	if got != connection.Path {
		t.Fatalf("resolved %q, want the real enroll derivation %q", got, connection.Path)
	}
}

func TestResolveConnectionConfigPathNoProjectFound(t *testing.T) {
	t.Setenv("AGENT_COMMONS_CONNECTION", "")
	dir := t.TempDir()
	_, err := resolveConnectionConfigPath("", dir)
	if err == nil {
		t.Fatal("expected an error when no manifest exists anywhere upward")
	}
	if !strings.Contains(err.Error(), "no Agent Commons project found") || !strings.Contains(err.Error(), "agent-commons init") {
		t.Fatalf("error does not name the failure and remedy: %v", err)
	}
}

func TestResolveConnectionConfigPathManifestUnreadableOrMalformed(t *testing.T) {
	t.Setenv("AGENT_COMMONS_CONNECTION", "")
	root := t.TempDir()
	projectDir := filepath.Join(root, ".agent-commons")
	if err := os.MkdirAll(projectDir, 0700); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(projectDir, "project.json")
	if err := os.WriteFile(manifestPath, []byte("not json"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := resolveConnectionConfigPath("", root)
	if err == nil {
		t.Fatal("expected an error for malformed manifest")
	}
	if !strings.Contains(err.Error(), manifestPath) {
		t.Fatalf("error does not name the failing manifest path: %v", err)
	}
}

func TestResolveConnectionConfigPathNotEnrolled(t *testing.T) {
	t.Setenv("AGENT_COMMONS_CONNECTION", "")
	root := t.TempDir()
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	state := t.TempDir()
	if err := os.Chmod(state, 0700); err != nil {
		t.Fatal(err)
	}
	writeProjectManifest(t, root, projectDefaults{Version: 1, ProjectRoot: root, Name: "lead", Role: "lead", State: state})
	expected, err := prepareEnrollment(onboardingOptions{Name: "lead", Role: "lead", Target: root, State: state})
	if err != nil {
		t.Fatal(err)
	}
	_, err = resolveConnectionConfigPath("", root)
	if err == nil {
		t.Fatal("expected an error when the project is initialized but this role is not enrolled")
	}
	if !strings.Contains(err.Error(), "not enrolled") || !strings.Contains(err.Error(), expected.Path) {
		t.Fatalf("error does not name the derived connection path %q: %v", expected.Path, err)
	}
}

func TestResolveConnectionConfigPathUnrecognizedVersion(t *testing.T) {
	t.Setenv("AGENT_COMMONS_CONNECTION", "")
	root := t.TempDir()
	state := t.TempDir()
	writeProjectManifest(t, root, projectDefaults{Version: 2, ProjectRoot: root, Name: "lead", Role: "lead", State: state})
	_, err := resolveConnectionConfigPath("", root)
	if err == nil {
		t.Fatal("expected an error for an unrecognized manifest version")
	}
	if !strings.Contains(err.Error(), "unrecognized version") {
		t.Fatalf("error does not describe the version problem: %v", err)
	}
}
