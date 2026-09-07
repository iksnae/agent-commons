// SPDX-License-Identifier: MPL-2.0

package codexrpc

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"
)

func TestCanceledWaitingCallDoesNotCloseActiveCall(t *testing.T) {
	clientSide, serverSide := net.Pipe()
	defer serverSide.Close()
	client := New(clientSide)
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := client.Call(ctx, "thread/read", struct{}{}); done <- err }()
	var request map[string]any
	if err := json.NewDecoder(serverSide).Decode(&request); err != nil {
		t.Fatal(err)
	}
	waiting, stopWaiting := context.WithCancel(context.Background())
	stopWaiting()
	if _, err := client.Call(waiting, "thread/start", struct{}{}); err == nil {
		t.Fatal("canceled waiting call succeeded")
	}
	if err := json.NewEncoder(serverSide).Encode(map[string]any{"id": request["id"], "result": struct{}{}}); err != nil {
		t.Fatal("waiting cancellation closed active transport", err)
	}
	if err := <-done; err != nil {
		t.Fatal("active call failed after waiting cancellation", err)
	}
}

type observedWrite struct {
	net.Conn
	started chan struct{}
}

func (w observedWrite) Write(data []byte) (int, error) {
	close(w.started)
	return w.Conn.Write(data)
}

func TestCancellationInterruptsBlockedRequestWrite(t *testing.T) {
	clientSide, serverSide := net.Pipe()
	defer serverSide.Close()
	stream := observedWrite{Conn: clientSide, started: make(chan struct{})}
	client := New(stream)
	defer client.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := client.Call(ctx, "thread/start", struct{}{}); done <- err }()
	select {
	case <-stream.started:
	case <-time.After(time.Second):
		t.Fatal("write never started")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("canceled write succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation did not interrupt write")
	}
}
