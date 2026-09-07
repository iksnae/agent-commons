// SPDX-License-Identifier: MPL-2.0

package codexlaunch

import (
	"encoding/json"
	"errors"
	"regexp"
)

var threadID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func rootIdentity(data json.RawMessage, target, expected string) (string, error) {
	var response struct {
		Thread map[string]json.RawMessage `json:"thread"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return "", errors.New("invalid native thread metadata")
	}
	var id, cwd string
	if json.Unmarshal(response.Thread["id"], &id) != nil || json.Unmarshal(response.Thread["cwd"], &cwd) != nil {
		return "", errors.New("native thread identity missing")
	}
	if !threadID.MatchString(id) || cwd != target || (expected != "" && id != expected) {
		return "", errors.New("native thread differs from prepared identity or target")
	}
	for _, ancestry := range []string{"forkedFromId", "parentThreadId"} {
		if string(response.Thread[ancestry]) != "null" {
			return "", errors.New("native root ancestry missing or nonempty")
		}
	}
	return id, nil
}
