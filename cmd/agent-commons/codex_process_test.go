// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agentcommons/internal/codexlaunch"
)

func TestCodexBootstrapCancellationStopsDescendantDuringInitialize(t *testing.T) {
	scope, listener := codexProcessFixture(t, "hang")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		process, err := startCodexBootstrap(ctx, scope)
		if process != nil {
			_ = process.Close()
		}
		result <- err
	}()
	child := acceptCodexDescendant(t, listener)
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("initialization cancellation returned %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("initialization did not stop after cancellation")
	}
	assertCodexDescendantStopped(t, child)
}

func TestCodexBootstrapCloseStopsDescendantAfterInitialize(t *testing.T) {
	scope, listener := codexProcessFixture(t, "ready")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startupLimit := time.AfterFunc(5*time.Second, cancel)
	defer startupLimit.Stop()
	process, err := startCodexBootstrap(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	defer process.Close()
	if !startupLimit.Stop() {
		t.Fatal("startup deadline expired")
	}
	// Only Close may end the process during the assertion; a parent deadline
	// would hide a broken Close by killing the descendant for us.
	child := acceptCodexDescendant(t, listener)
	if err = process.Close(); err != nil {
		t.Fatal(err)
	}
	assertCodexDescendantStopped(t, child)
	if err = process.Close(); err != nil {
		t.Fatal("repeated close failed", err)
	}
}

func codexProcessFixture(t *testing.T, mode string) (codexlaunch.Scope, *net.TCPListener) {
	t.Helper()
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	if err = listener.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	quoted := "'" + strings.ReplaceAll(executable, "'", "'\"'\"'") + "'"
	script := "#!/bin/sh\nexec " + quoted + " -test.run '^TestCodexProcessFixture$' -- " + mode + " " + listener.Addr().String() + "\n"
	if err = os.WriteFile(filepath.Join(bin, "codex"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return codexlaunch.Scope{Identity: "fixture", Target: t.TempDir(), Home: t.TempDir()}, listener
}

func acceptCodexDescendant(t *testing.T, listener *net.TCPListener) net.Conn {
	t.Helper()
	conn, err := listener.Accept()
	if err != nil {
		t.Fatal("descendant did not start", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err = conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var ready [1]byte
	if _, err = io.ReadFull(conn, ready[:]); err != nil || ready[0] != 'R' {
		t.Fatal("descendant did not report readiness", err)
	}
	return conn
}

func assertCodexDescendantStopped(t *testing.T, child net.Conn) {
	t.Helper()
	var extra [1]byte
	if _, err := child.Read(extra[:]); !errors.Is(err, io.EOF) {
		t.Fatalf("descendant retained its connection after shutdown: %v", err)
	}
}
