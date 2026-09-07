// SPDX-License-Identifier: MPL-2.0

package transport

import "testing"

func TestTeamWorkScopeIsDiscoverableWithoutBroadeningOtherTools(t *testing.T) {
	expected := map[string]bool{"context.put": true, "context.get": true, "tasks.assign": true, "messages.send": true}
	for _, tool := range toolDefinitions() {
		props := tool.InputSchema["properties"].(map[string]any)
		_, hasScope := props["teamId"]
		if hasScope != expected[tool.Name] {
			t.Fatal("unexpected team scope schema", tool.Name)
		}
		if tool.InputSchema["additionalProperties"] != false {
			t.Fatal("schema became permissive", tool.Name)
		}
	}
}
