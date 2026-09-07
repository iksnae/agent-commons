// SPDX-License-Identifier: MPL-2.0

package codexrpc

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"
)

func TestCallSkipsNotificationsAndReturnsMatchingResult(t *testing.T) {
	clientSide, serverSide := net.Pipe()
	defer serverSide.Close()
	client := New(clientSide)
	defer client.Close()
	go func() {
		var request map[string]any
		if json.NewDecoder(serverSide).Decode(&request) != nil {
			return
		}
		encoder := json.NewEncoder(serverSide)
		_ = encoder.Encode(map[string]any{"method": "thread/started", "params": map[string]string{}})
		_ = encoder.Encode(map[string]any{"id": request["id"], "result": map[string]string{"id": "root"}})
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result, err := client.Call(ctx, "thread/start", struct{}{})
	if err != nil || string(result) != `{"id":"root"}` {
		t.Fatal("matching response not returned", string(result), err)
	}
}

func TestCallCancellationClosesBlockedTransport(t *testing.T) {
	clientSide, serverSide := net.Pipe()
	defer serverSide.Close()
	client := New(clientSide)
	defer client.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	received := make(chan struct{})
	go func() {
		var request any
		if json.NewDecoder(serverSide).Decode(&request) == nil {
			close(received)
		}
	}()
	done := make(chan error, 1)
	go func() { _, err := client.Call(ctx, "thread/start", struct{}{}); done <- err }()
	select {
	case <-received:
	case <-time.After(time.Second):
		t.Fatal("request did not reach server")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("canceled call succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("canceled call blocked on transport")
	}
}

func TestCallRejectsProtocolViolationsWithoutLeakingPayload(t *testing.T) {
	for _, reply := range []string{
		`{"id":999,"result":{}}`,
		`{"id":1,"method":"approval/request","params":{"secret":"sensitive"}}`,
		`{"id":1,"error":{"code":-1,"message":"sensitive"}}`,
		`{"id":1,"result":{},"error":{}}`,
		`{"id":1}`,
		`{"id":1,"result":"` + strings.Repeat("sensitive", 140000) + `"}`,
	} {
		clientSide, serverSide := net.Pipe()
		client := New(clientSide)
		go func() {
			defer serverSide.Close()
			var request any
			if json.NewDecoder(serverSide).Decode(&request) == nil {
				_, _ = serverSide.Write([]byte(reply + "\n"))
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_, err := client.Call(ctx, "thread/start", struct{}{})
		cancel()
		_ = client.Close()
		if err == nil || strings.Contains(err.Error(), "sensitive") {
			t.Fatal("invalid reply accepted or payload leaked", err)
		}
	}
}
