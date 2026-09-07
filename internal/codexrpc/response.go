// SPDX-License-Identifier: MPL-2.0

package codexrpc

import (
	"encoding/json"
	"errors"
	"strconv"
)

type envelope struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Result json.RawMessage `json:"result"`
	Error  json.RawMessage `json:"error"`
}

func (c *Client) response(id uint64) (json.RawMessage, error) {
	for count := 0; count < 1024 && c.lines.Scan(); count++ {
		var event envelope
		if err := json.Unmarshal(c.lines.Bytes(), &event); err != nil {
			return nil, errors.New("invalid Codex response frame")
		}
		if len(event.ID) == 0 && event.Method != "" && len(event.Result) == 0 && len(event.Error) == 0 {
			continue
		}
		if event.Method != "" || string(event.ID) != strconv.FormatUint(id, 10) {
			return nil, errors.New("unexpected Codex response or server request; no approval granted")
		}
		if len(event.Error) != 0 {
			return nil, errors.New("Codex request rejected; inspect native diagnostics privately")
		}
		if len(event.Result) == 0 {
			return nil, errors.New("Codex response has no result")
		}
		return event.Result, nil
	}
	return nil, errors.New("Codex response stream ended or exceeded frame/notification limit")
}
