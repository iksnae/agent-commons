// SPDX-License-Identifier: MPL-2.0

package transport

func withTeamWork(tools []tool) []tool {
	for i := range tools {
		switch tools[i].Name {
		case "messages.send", "tasks.assign", "context.put", "context.get":
			properties := tools[i].InputSchema["properties"].(map[string]any)
			properties["teamId"] = map[string]any{"type": "string"}
			tools[i].Description += " Optional teamId selects joined-team scope; omit for project scope. Scoped work requires joined participants and keeps context and returns in that team."
		}
	}
	return tools
}
