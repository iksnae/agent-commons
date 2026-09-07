// SPDX-License-Identifier: MPL-2.0

package integration

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestPortableManifestKeepsScopedConnectionCommand(t *testing.T) {
	read := func(name string) map[string]any {
		t.Helper()
		data, err := os.ReadFile("../plugins/agent-commons/" + name)
		if err != nil {
			t.Fatal(err)
		}
		var value map[string]any
		if err := json.Unmarshal(data, &value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	manifest := read("plugin.json")
	if manifest["$schema"] != "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json" || manifest["name"] != "agent-commons" || manifest["license"] != "MPL-2.0" {
		t.Fatal("portable identity/schema/license changed", manifest)
	}
	want := map[string]any{"command": "agent-commons", "args": []any{"connect-mcp"}}
	if !reflect.DeepEqual(read(".mcp.json"), map[string]any{"mcpServers": map[string]any{"agent-commons": want}}) {
		t.Fatal("native MCP must use only the scoped connection command")
	}
	want["type"] = "stdio"
	if !reflect.DeepEqual(read("mcp.json"), map[string]any{
		"$schema":    "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
		"mcpServers": map[string]any{"agent-commons": want},
	}) {
		t.Fatal("portable MCP must use the same scoped command without bundled credentials")
	}
}
