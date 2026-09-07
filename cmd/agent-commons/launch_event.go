// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
)

type launchEvent struct {
	Event     string `json:"hook_event_name"`
	Session   string `json:"session_id"`
	Directory string `json:"cwd"`
	Source    string `json:"source"`
	AgentID   string `json:"agent_id"`
	AgentType string `json:"agent_type"`
}

func readLaunchEvent(in io.Reader) (launchEvent, error) {
	var event launchEvent
	data, err := io.ReadAll(io.LimitReader(in, (64<<10)+1))
	if err != nil {
		return event, err
	}
	if len(data) > 64<<10 {
		return event, fmt.Errorf("launch event exceeds 64 KiB")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err = decoder.Decode(&event); err != nil {
		return event, err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return event, fmt.Errorf("one launch event required")
	}
	if event.Event != "SessionStart" || event.AgentID != "" || event.Session == "" || len(event.Session) > 256 || !filepath.IsAbs(event.Directory) {
		return event, fmt.Errorf("exact primary-session launch context required")
	}
	switch event.Source {
	case "startup", "resume", "clear", "compact":
	default:
		return event, fmt.Errorf("forked or unknown launch sources require separate enrollment")
	}
	return event, nil
}
