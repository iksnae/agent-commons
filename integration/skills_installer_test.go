// SPDX-License-Identifier: MPL-2.0

package integration

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

const skillsInstallerVersion = "1.5.24"

func TestVercelSkillsInstallerPreservesPortableSkill(t *testing.T) {
	if os.Getenv("AGENT_COMMONS_SKILLS_INSTALLER") != "1" {
		t.Skip("opt-in: downloads pinned Vercel installer; no native agents or models")
	}
	npx, err := exec.LookPath("npx")
	if err != nil {
		t.Fatal(err)
	}
	source, err := filepath.Abs("../plugins/agent-commons")
	if err != nil {
		t.Fatal(err)
	}
	fixture := t.TempDir()
	project := filepath.Join(fixture, "project")
	if err = os.Mkdir(project, 0700); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, npx, append([]string{"--yes", "skills@" + skillsInstallerVersion}, args...)...)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		cmd.Cancel = func() error {
			err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			if errors.Is(err, syscall.ESRCH) {
				return os.ErrProcessDone
			}
			return err
		}
		cmd.WaitDelay = time.Second
		cmd.Dir = project
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + fixture, "XDG_CONFIG_HOME=" + filepath.Join(fixture, "config"), "CODEX_HOME=" + filepath.Join(fixture, "codex"), "npm_config_cache=" + filepath.Join(fixture, "npm-cache"), "DISABLE_TELEMETRY=1", "DO_NOT_TRACK=1", "CI=1"}
		for _, name := range []string{"NODE_EXTRA_CA_CERTS", "SSL_CERT_FILE", "SSL_CERT_DIR", "npm_config_cafile"} {
			if value := os.Getenv(name); value != "" {
				cmd.Env = append(cmd.Env, name+"="+value)
			}
		}
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("skills %v: %v\n%s", args, err, output)
		}
	}
	run("add", source, "--skill", "agent-commons", "--agent", "claude-code", "codex", "pi", "hermes-agent", "--copy", "--yes")
	paths := []string{".claude/skills/agent-commons", ".agents/skills/agent-commons", ".pi/skills/agent-commons", ".hermes/skills/agent-commons"}
	for _, path := range paths {
		compareInstalledSkill(t, filepath.Join(source, "skills/agent-commons"), filepath.Join(project, path))
	}
	// Installing instructions must not register native hooks, MCP, or a service.
	for _, path := range []string{".mcp.json", ".claude/settings.json", ".pi/settings.json", ".hermes/config.yaml"} {
		if _, err := os.Lstat(filepath.Join(project, path)); !os.IsNotExist(err) {
			t.Fatal("unexpected native configuration", path, err)
		}
	}
	run("remove", "agent-commons", "--agent", "claude-code", "codex", "pi", "hermes-agent", "--yes")
	for _, path := range paths {
		if _, err := os.Lstat(filepath.Join(project, path)); !os.IsNotExist(err) {
			t.Fatal("skill not removed", path, err)
		}
	}
}

func compareInstalledSkill(t *testing.T, source, installed string) {
	t.Helper()
	err := filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		want, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		got, err := os.ReadFile(filepath.Join(installed, relative))
		if err != nil {
			return err
		}
		if !bytes.Equal(got, want) {
			t.Errorf("installed support file differs: %s", relative)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
