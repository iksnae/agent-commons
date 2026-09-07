// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCommandErrors(t *testing.T) {
	for _, args := range [][]string{{}, {"unknown"}, {"mcp"}, {"discover", "--runtime", "invalid"}} {
		var out, errOut bytes.Buffer
		if err := run(context.Background(), args, strings.NewReader(""), &out, &errOut); err == nil {
			t.Fatalf("expected error for %v", args)
		}
	}
}

func TestServeCallAndShutdown(t *testing.T) {
	state, err := os.MkdirTemp("", "ac-cli-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(state)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, []string{"serve", "--state", state}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	}()
	defer func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(5 * time.Second):
			t.Error("serve failed to stop")
		}
	}()
	var out, errOut bytes.Buffer
	deadline := time.Now().Add(3 * time.Second)
	for {
		out.Reset()
		err = run(ctx, []string{"call", "--state", state, "sessions.list", "{}"}, strings.NewReader(""), &out, &errOut)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if strings.TrimSpace(out.String()) != "[]" {
		t.Fatalf("unexpected registry: %s", out.String())
	}
	out.Reset()
	if err = run(ctx, []string{"call", "--state", state, "sessions.list", "broken"}, strings.NewReader(""), &out, &errOut); err == nil {
		t.Fatal("invalid call JSON accepted")
	}
}
func TestPrivateCredential(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte("secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	token, err := readToken(path)
	if err != nil || token != "secret" {
		t.Fatalf("read failed %v", err)
	}
	if err = os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err = readToken(path); err == nil {
		t.Fatal("public credential accepted")
	}
	link := filepath.Join(t.TempDir(), "link")
	if err = os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err = readToken(link); err == nil {
		t.Fatal("symlink accepted")
	}
}
func TestMCPProtocolOutputIsClean(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	err := run(context.Background(), []string{"mcp", "--token-file", path}, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`), &out, &errOut)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), `{"jsonrpc":"2.0"`) {
		t.Fatal(out.String())
	}
	if strings.Contains(out.String(), "secret") {
		t.Fatal("credential leaked")
	}
}
