// SPDX-License-Identifier: MPL-2.0

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// resolveConnectionConfigPath finds the private connection file a role
// command should open, in strict precedence:
//
//  1. explicit (an already-parsed --config flag value) always wins.
//  2. AGENT_COMMONS_CONNECTION from the environment.
//  3. an upward walk from startDir for .agent-commons/project.json, deriving
//     the connection path the same way `enroll` would have written it for
//     that manifest's projectRoot/name/role (see deriveEnrolledConnectionPath,
//     which mirrors prepareEnrollment in enrollment.go).
//
// Every negative outcome names what is missing and what to do about it;
// generic "not found" errors are never returned from here.
func resolveConnectionConfigPath(explicit, startDir string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if env := os.Getenv("AGENT_COMMONS_CONNECTION"); env != "" {
		return env, nil
	}
	manifestPath, err := findProjectManifest(startDir)
	if err != nil {
		return "", fmt.Errorf("locating .agent-commons/project.json above %s: %w", startDir, err)
	}
	if manifestPath == "" {
		return "", fmt.Errorf("no Agent Commons project found walking up from %s; run `agent-commons init` to establish one, or pass --config, or set AGENT_COMMONS_CONNECTION", startDir)
	}
	var manifest projectDefaults
	if err := privateRead(manifestPath, &manifest); err != nil {
		return "", fmt.Errorf("project manifest %s is unreadable or malformed: %w", manifestPath, err)
	}
	if manifest.Version != 1 {
		return "", fmt.Errorf("project manifest %s has unrecognized version %d", manifestPath, manifest.Version)
	}
	config, err := deriveEnrolledConnectionPath(manifest)
	if err != nil {
		return "", fmt.Errorf("project manifest %s: %w", manifestPath, err)
	}
	if _, statErr := os.Lstat(config); errors.Is(statErr, os.ErrNotExist) {
		return "", fmt.Errorf("project %s is initialized (manifest %s) but %s/%s is not enrolled; no connection file at %s; run `agent-commons enroll` for this role", manifest.ProjectRoot, manifestPath, manifest.Name, manifest.Role, config)
	} else if statErr != nil {
		return "", fmt.Errorf("checking enrolled connection %s: %w", config, statErr)
	}
	return config, nil
}

// deriveEnrolledConnectionPath reproduces prepareEnrollment's derivation
// (enrollment.go) from a project manifest instead of onboardingOptions.
func deriveEnrolledConnectionPath(manifest projectDefaults) (string, error) {
	if manifest.ProjectRoot == "" || manifest.Name == "" || manifest.Role == "" || manifest.State == "" {
		return "", errors.New("manifest is missing required projectRoot, name, role or state field")
	}
	identity := "agent-" + identityDigest(manifest.ProjectRoot+"\x00"+manifest.Name+"\x00"+manifest.Role)
	stem := identityDigest(identity)
	return filepath.Join(manifest.State, "connection-"+stem+".json"), nil
}

// findProjectManifest walks upward from startDir looking for
// .agent-commons/project.json, never past the filesystem root and never
// following a symlink out of the tree: startDir is canonicalized once, and
// the walk afterward is pure path arithmetic on that canonical path, plus an
// Lstat (not Stat) on each candidate manifest so a symlinked manifest is
// skipped rather than followed.
func findProjectManifest(startDir string) (string, error) {
	abs, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	dir := resolved
	for {
		candidate := filepath.Join(dir, ".agent-commons", "project.json")
		info, statErr := os.Lstat(candidate)
		if statErr == nil && info.Mode().IsRegular() {
			return candidate, nil
		}
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return "", statErr
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		dir = parent
	}
}
