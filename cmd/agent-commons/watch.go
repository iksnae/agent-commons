// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"time"

	"agentcommons/internal/core"
)

type inboxReader func(context.Context) ([]core.Delivery, error)

type availability struct {
	Type      string `json:"type"`
	MessageID string `json:"messageId"`
	Recipient string `json:"recipient"`
}

const maxWatchMessages = 10000

// watchInbox reports availability, not reading/handling. Unacknowledged messages
// replay on restart; downstream consumers must deduplicate by messageId.
func watchInbox(ctx context.Context, interval time.Duration, once bool, read inboxReader, out io.Writer) error {
	if interval < 100*time.Millisecond {
		return errors.New("watch interval must be at least 100ms")
	}
	seen := map[string]bool{}
	encoder := json.NewEncoder(out)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		messages, err := read(ctx)
		if err != nil {
			return err
		} // Fail visibly; an unavailable inbox is not empty.
		if len(messages) > maxWatchMessages {
			return errors.New("inbox exceeds watcher notification budget; no messages acknowledged")
		}
		present := map[string]bool{}
		emitted := false
		batch := []availability{}
		for _, m := range messages {
			if m.Acknowledged {
				continue
			}
			present[m.ID] = true
			if seen[m.ID] {
				continue
			}
			// No peer text or credentials in the notification. Fetch content separately.
			batch = append(batch, availability{"inbox.available", m.ID, m.To})
			emitted = true
		}
		if sink, ok := out.(interface{ Notify([]availability) error }); ok {
			if err := sink.Notify(batch); err != nil {
				return err
			}
		} else {
			for _, event := range batch {
				if err := encoder.Encode(event); err != nil {
					return err
				}
			}
		}
		seen = present
		if once && emitted {
			return nil
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func runWatch(ctx context.Context, args []string, out, errOut io.Writer) error {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	fs.SetOutput(errOut)
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	state := fs.String("state", filepath.Join(home, ".local", "state", "agent-commons"), "private service state directory")
	socket := fs.String("socket", "", "service Unix socket")
	tokenFile := fs.String("token-file", "", "required scoped identity credential; no operator fallback")
	interval := fs.Duration("interval", 2*time.Second, "mechanical inbox check interval, minimum 100ms")
	once := fs.Bool("once", false, "exit after first nonempty batch of notifications")
	timeout := fs.Duration("timeout", 0, "overall deadline; zero waits until canceled")
	wakeThread := fs.String("codex-thread", "", "explicitly bind this inbox to an existing Codex thread UUID and queue fixed arrival signals")
	if err := parseFlags(fs, args, out); err != nil {
		return err
	}
	if fs.NArg() != 0 || *tokenFile == "" || *timeout < 0 {
		return errors.New("watch requires --token-file and no positional arguments; timeout must be nonnegative")
	}
	if *socket == "" {
		*socket = filepath.Join(*state, "service.sock")
	}
	token, err := readToken(*tokenFile)
	if err != nil {
		return err
	}
	if *timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}
	// watch is human-facing and long-lived: an operator watching a service that
	// is not running is the reported condition verbatim. Both dials go through
	// rpcCall so a failure comes back classified, and run renders it once.
	client := rpcClient{socket: *socket, token: token, state: *state}
	if *wakeThread != "" {
		identity, err := rpcCall[struct {
			Identity string `json:"identity"`
		}](ctx, client, "sessions.capabilities", struct{}{})
		if err != nil {
			return err
		}
		if identity.Identity == "" || identity.Identity == "operator" {
			return errors.New("wake requires a registered session identity")
		}
		canonicalSocket, err := filepath.EvalSymlinks(*socket)
		if err != nil {
			return err
		}
		w, err := openWake(ctx, *state, *wakeThread, canonicalSocket+"\x00"+identity.Identity, out)
		if err != nil {
			return err
		}
		defer w.lock.Close()
		out = w
	}
	return watchInbox(ctx, *interval, *once, func(ctx context.Context) ([]core.Delivery, error) {
		var messages []core.Delivery
		cursor := ""
		for page := 0; page < 1000; page++ {
			result, err := rpcCall[struct {
				Messages   []core.Delivery `json:"messages"`
				NextCursor string          `json:"nextCursor"`
			}](ctx, client, "inbox.page",
				map[string]any{"cursor": cursor, "limit": 100, "unreadOnly": true})
			if err != nil {
				return nil, err
			}
			if len(result.Messages) > maxWatchMessages-len(messages) {
				return nil, errors.New("inbox exceeds watcher notification budget; no messages acknowledged")
			}
			messages = append(messages, result.Messages...)
			if result.NextCursor == "" {
				return messages, nil
			}
			if result.NextCursor == cursor {
				return nil, errors.New("inbox cursor did not advance")
			}
			cursor = result.NextCursor
		}
		return nil, errors.New("inbox exceeds watcher page budget")
	}, out)
}
