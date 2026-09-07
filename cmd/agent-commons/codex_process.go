// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"agentcommons/internal/codexlaunch"
	"agentcommons/internal/codexrpc"
)

type ownedCodexBootstrap struct {
	*codexrpc.Client
	command *exec.Cmd
	cancel  context.CancelFunc
	once    sync.Once
}

func startCodexBootstrap(ctx context.Context, scope codexlaunch.Scope) (codexBootstrap, error) {
	ctx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(ctx, "codex", "app-server", "--stdio", "--config", "features.hooks=false")
	cmd.Dir = scope.Target
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME"), "TMPDIR=" + os.Getenv("TMPDIR"), "CODEX_HOME=" + scope.Home}
	cmd.Stderr = io.Discard
	cmd.WaitDelay = time.Second
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if err == syscall.ESRCH {
			return os.ErrProcessDone
		}
		return err
	}
	in, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		_ = in.Close()
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		cancel()
		_ = in.Close()
		_ = out.Close()
		return nil, err
	}
	p := &ownedCodexBootstrap{Client: codexrpc.New(codexrpc.Pipes(in, out)), command: cmd, cancel: cancel}
	_, err = p.Call(ctx, "initialize", map[string]any{"clientInfo": map[string]string{"name": "agent-commons", "version": "0.1.0"}, "capabilities": map[string]bool{"experimentalApi": true}})
	if err == nil {
		err = p.Notify(ctx, "initialized")
	}
	if err != nil {
		_ = p.Close()
		return nil, err
	}
	return p, nil
}

func (p *ownedCodexBootstrap) Close() error {
	p.once.Do(func() { p.cancel(); _ = p.Client.Close(); _ = p.command.Wait() })
	return nil
}
