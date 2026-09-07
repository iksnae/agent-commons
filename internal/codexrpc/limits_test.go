// SPDX-License-Identifier: MPL-2.0

package codexrpc

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

type memoryStream struct {
	io.Reader
	writes bytes.Buffer
	closed bool
}

func (s *memoryStream) Write(data []byte) (int, error) { return s.writes.Write(data) }
func (s *memoryStream) Close() error                   { s.closed = true; return nil }

func TestOversizedRequestDoesNotWrite(t *testing.T) {
	stream := &memoryStream{Reader: strings.NewReader("")}
	client := New(stream)
	_, err := client.Call(context.Background(), "thread/start", strings.Repeat("x", maxFrame))
	if err == nil || stream.writes.Len() != 0 || !stream.closed {
		t.Fatal("oversized request sent or connection left usable")
	}
}

func TestNotificationFloodIsBounded(t *testing.T) {
	stream := &memoryStream{Reader: strings.NewReader(strings.Repeat("{\"method\":\"progress\"}\n", 1024))}
	client := New(stream)
	if _, err := client.Call(context.Background(), "thread/read", struct{}{}); err == nil || !stream.closed {
		t.Fatal("notification flood did not close connection")
	}
}

func TestFailedConnectionCannotSendAnotherRequest(t *testing.T) {
	stream := &memoryStream{Reader: strings.NewReader("invalid\n")}
	client := New(stream)
	if _, err := client.Call(context.Background(), "thread/read", struct{}{}); err == nil {
		t.Fatal("malformed response accepted")
	}
	written := stream.writes.Len()
	if _, err := client.Call(context.Background(), "thread/start", struct{}{}); err == nil || stream.writes.Len() != written {
		t.Fatal("failed connection sent another request")
	}
}
