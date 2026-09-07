// SPDX-License-Identifier: MPL-2.0

// Package codexrpc provides a bounded, sequential Codex bootstrap connection.
// It does not start processes, trust hooks, grant approvals or retry requests.
// Notifications are discarded; turn supervision needs a separate event consumer.
package codexrpc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
)

const maxFrame = 1 << 20

type Client struct {
	stream io.ReadWriteCloser
	lines  *bufio.Scanner
	gate   chan struct{}
	closed chan struct{}
	once   sync.Once
	id     uint64
}

// New takes ownership of a stream whose Close interrupts blocked reads/writes.
func New(stream io.ReadWriteCloser) *Client {
	scanner := bufio.NewScanner(stream)
	scanner.Buffer(make([]byte, 4096), maxFrame)
	return &Client{stream: stream, lines: scanner, gate: make(chan struct{}, 1), closed: make(chan struct{})}
}

// Close invalidates the connection. It is safe to call concurrently or repeatedly.
func (c *Client) Close() error {
	var err error
	c.once.Do(func() { close(c.closed); err = c.stream.Close() })
	return err
}

// Call sends one request. Cancellation or protocol failure closes the connection;
// a failed call may already have acted and must not be retried automatically.
func (c *Client) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	var result json.RawMessage
	err := c.operate(ctx, func() error {
		c.id++
		if err := c.send(map[string]any{"id": c.id, "method": method, "params": params}); err != nil {
			return err
		}
		var err error
		result, err = c.response(c.id)
		return err
	})
	return result, err
}

// Notify sends a notification without waiting for a response.
func (c *Client) Notify(ctx context.Context, method string) error {
	return c.operate(ctx, func() error { return c.send(map[string]string{"method": method}) })
}

func (c *Client) operate(ctx context.Context, work func() error) error {
	select {
	case c.gate <- struct{}{}:
		defer func() { <-c.gate }()
	case <-ctx.Done():
		return ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case <-c.closed:
		return errors.New("Codex connection closed")
	default:
	}
	stop := context.AfterFunc(ctx, func() { _ = c.Close() })
	defer stop()
	err := work()
	if err != nil {
		_ = c.Close()
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

func (c *Client) send(value any) error {
	data, err := json.Marshal(value)
	if err != nil || len(data)+1 >= maxFrame {
		return errors.New("Codex request is invalid or exceeds frame limit")
	}
	data = append(data, '\n')
	written, err := c.stream.Write(data)
	if err != nil || written != len(data) {
		return errors.New("Codex request write failed; external outcome may be uncertain")
	}
	return nil
}
