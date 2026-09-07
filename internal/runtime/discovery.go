package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"
)

// DiscoverCodex connects only to an already-running daemon; it never starts one.
func DiscoverCodex(ctx context.Context, target string) ([]Discovered, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "codex", "app-server", "proxy")
	cmd.Dir = target
	cmd.WaitDelay = 2 * time.Second
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderr boundedBuffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("Codex discovery unavailable: %w", err)
	}
	defer func() { cancel(); _ = in.Close(); _ = cmd.Wait() }()
	encoder := json.NewEncoder(in)
	scanner := bufio.NewScanner(out)
	scanner.Buffer(make([]byte, 4096), outputLimit)
	request := func(id int, method string, params any) (json.RawMessage, error) {
		if err := encoder.Encode(map[string]any{"id": id, "method": method, "params": params}); err != nil {
			return nil, err
		}
		for scanner.Scan() {
			var event struct {
				ID     int             `json:"id"`
				Result json.RawMessage `json:"result"`
				Error  json.RawMessage `json:"error"`
			}
			if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
				return nil, err
			}
			if event.ID != id {
				continue
			}
			if len(event.Error) > 0 {
				return nil, fmt.Errorf("Codex discovery RPC rejected %s", method)
			}
			return event.Result, nil
		}
		return nil, fmt.Errorf("Codex discovery unavailable: daemon stream closed (%v)", scanner.Err())
	}
	if _, err := request(1, "initialize", map[string]any{"clientInfo": map[string]string{"name": "agent-commons", "version": "0.1.0"}, "capabilities": map[string]bool{"experimentalApi": true}}); err != nil {
		return nil, err
	}
	if err := encoder.Encode(map[string]string{"method": "initialized"}); err != nil {
		return nil, err
	}
	var result []Discovered
	cursor := ""
	for page := 0; page < 100; page++ {
		params := map[string]any{"limit": 100, "cwd": target}
		if cursor != "" {
			params["cursor"] = cursor
		}
		raw, err := request(page+2, "thread/list", params)
		if err != nil {
			return nil, err
		}
		var response struct {
			Data []struct {
				ID     string `json:"id"`
				CWD    string `json:"cwd"`
				Name   string `json:"name"`
				Status struct {
					Type string `json:"type"`
				} `json:"status"`
			} `json:"data"`
			NextCursor string `json:"nextCursor"`
		}
		if err := json.Unmarshal(raw, &response); err != nil {
			return nil, err
		}
		for _, thread := range response.Data {
			if filepath.Clean(thread.CWD) != filepath.Clean(target) {
				continue
			}
			result = append(result, Discovered{CWD: thread.CWD, SessionID: thread.ID, Name: thread.Name, Status: thread.Status.Type, Kind: "codex-thread"})
		}
		if response.NextCursor == "" {
			return result, nil
		}
		cursor = response.NextCursor
	}
	return nil, fmt.Errorf("Codex discovery exceeded pagination limit")
}
