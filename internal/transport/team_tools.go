// SPDX-License-Identifier: MPL-2.0

package transport

func teamTools() []tool {
	return []tool{
		{"teams.create", "Operator only: create an immutable project team brief. First creation advances state to schema 2; back up before use. Does not authorize work.", schema([]string{"id", "target", "title", "text"}, map[string]string{"id": "string", "target": "string", "title": "string", "text": "string"})},
		{"teams.invite", "Operator only: invite an enrolled identity from this project; sends one notice for a new invitation.", schema([]string{"id", "target", "to"}, map[string]string{"id": "string", "target": "string", "to": "string"})},
		{"teams.revoke", "Operator only: revoke team access. Existing project-wide access and task duties are unchanged.", schema([]string{"id", "target", "to"}, map[string]string{"id": "string", "target": "string", "to": "string"})},
		{"teams.get", "Read an invited team's brief and joined roster. Does not acknowledge messages or grant work authority.", schema([]string{"id"}, map[string]string{"id": "string", "target": "string"})},
		{"teams.join", "Join or rejoin an invited project team. Preserves native session, role credentials and inbox history.", schema([]string{"id"}, map[string]string{"id": "string"})},
		{"teams.leave", "Leave a team roster; invitation remains valid until revoked. Task responsibilities and project-wide access remain unchanged.", schema([]string{"id"}, map[string]string{"id": "string"})},
	}
}
