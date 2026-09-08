// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"agentcommons/internal/harness"
)

type projectDefaults struct {
	Version     int    `json:"version"`
	ProjectRoot string `json:"projectRoot"`
	Team        string `json:"team"`
	Name        string `json:"name"`
	Role        string `json:"role"`
	Runtime     string `json:"runtime"`
	State       string `json:"state"`
}

func runInit(ctx context.Context, args []string, out, errOut io.Writer) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(errOut)
	target, _ := os.Getwd()
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	state := filepath.Join(home, ".local", "state", "agent-commons")
	name, role, team, runtimeName := "lead", "workspace-lead", "", "codex"
	fs.StringVar(&target, "target", target, "project/workspace directory (default current directory)")
	fs.StringVar(&state, "state", state, "private persistent service state directory")
	fs.StringVar(&name, "name", name, "stable agent name")
	fs.StringVar(&role, "role", role, "project role")
	fs.StringVar(&team, "team", team, "project team label")
	fs.StringVar(&runtimeName, "runtime", runtimeName, "runtime: "+runtimeVocabulary())
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || name == "" || role == "" {
		return errors.New("init requires --name and --role and no positional arguments")
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return err
	}
	target, err = filepath.EvalSymlinks(target)
	if err != nil {
		return err
	}
	if _, ok := harness.Lookup(runtimeName); !ok {
		return fmt.Errorf("runtime must be %s", runtimeVocabulary())
	}
	if err := os.MkdirAll(state, 0700); err != nil {
		return err
	}
	if err := os.Chmod(state, 0700); err != nil {
		return err
	}
	socket := filepath.Join(state, "service.sock")
	if !serviceReachable(socket) {
		binary, err := os.Executable()
		if err != nil {
			return err
		}
		cmd := exec.Command(binary, "serve", "--state", state)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, io.Discard, io.Discard
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("start service: %w", err)
		}
		_ = cmd.Process.Release()
	}
	deadline := time.Now().Add(3 * time.Second)
	for !serviceReachable(socket) && time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
	if !serviceReachable(socket) {
		return errors.New("service did not become reachable; inspect the private state directory")
	}
	options := onboardingOptions{Command: "enroll", State: state, Name: name, Role: role, Team: team, Target: target}
	connection, err := prepareEnrollment(options)
	if err != nil {
		return err
	}
	if _, err := connection.checkExisting(); err != nil {
		return err
	}
	if err := enrollAgent(ctx, options, io.Discard); err != nil {
		return err
	}
	data, err := json.MarshalIndent(projectDefaults{Version: 1, ProjectRoot: target, Team: team, Name: name, Role: role, Runtime: runtimeName, State: state}, "", "  ")
	if err != nil {
		return err
	}
	projectDir := filepath.Join(target, ".agent-commons")
	if err := os.MkdirAll(projectDir, 0700); err != nil {
		return err
	}
	manifest := filepath.Join(projectDir, "project.json")
	if err := os.WriteFile(manifest, append(data, '\n'), 0600); err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(map[string]string{"operation": "init", "target": target, "manifest": manifest, "config": connection.Path, "state": state, "runtime": runtimeName})
}

func serviceReachable(socket string) bool {
	conn, err := net.DialTimeout("unix", socket, 50*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
