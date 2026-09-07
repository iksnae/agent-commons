// SPDX-License-Identifier: MPL-2.0

package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestNativeHermesLoadsPortablePackage(t *testing.T) {
	if os.Getenv("AGENT_COMMONS_HERMES_LOADER") != "1" {
		t.Skip("opt-in: parses the bundle with an installed Hermes runtime; no CLI or models")
	}
	runtime := os.Getenv("AGENT_COMMONS_HERMES_ROOT")
	if !filepath.IsAbs(runtime) {
		t.Fatal("AGENT_COMMONS_HERMES_ROOT must be an absolute Hermes source directory")
	}
	python := filepath.Join(runtime, "venv/bin/python")
	fixture := t.TempDir()
	// Copy the bundle to prove the loader does not rely on checkout-relative files.
	source, err := filepath.Abs("../plugins/agent-commons")
	if err != nil {
		t.Fatal(err)
	}
	installed := filepath.Join(fixture, "package")
	if err := os.CopyFS(installed, os.DirFS(source)); err != nil {
		t.Fatal(err)
	}
	probe, err := filepath.Abs("hermes_loader_probe.py")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, python, "-I", "-B", probe, runtime, installed, filepath.Join(fixture, "data"))
	cmd.Dir = fixture
	cmd.Env = []string{"HOME=" + fixture, "HERMES_HOME=" + filepath.Join(fixture, "profile"), "PYTHONIOENCODING=utf-8"}
	cmd.WaitDelay = time.Second
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("native Hermes loader: %v\n%s", err, output)
	}
	t.Log(string(output))
}
