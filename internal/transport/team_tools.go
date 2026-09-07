// SPDX-License-Identifier: MPL-2.0

package transport

func teamTools() []tool {
	return []tool{
		{"teams.list", "List your invited, joined and left project teams, never uninvited or revoked teams. Limit 1..20, default 10; follow nextCursor. Operator must supply target. No inbox acknowledgement.", schema([]string{}, map[string]string{"target": "string", "cursor": "string", "limit": "integer"})},
		{"teams.create", "Operator only: create an immutable project team brief. First creation preserves a private core-state backup before advancing to schema 2. Full service backup remains separate. Does not authorize work.", schema([]string{"id", "target", "title", "text"}, map[string]string{"id": "string", "target": "string", "title": "string", "text": "string"})},
		{"teams.invite", "Operator only: invite an enrolled identity from this project; sends one notice for a new invitation.", schema([]string{"id", "target", "to"}, map[string]string{"id": "string", "target": "string", "to": "string"})},
		{"teams.revoke", "Operator only: revoke team access, blocking scoped work and triggering managed cancellation when observed. Existing project-wide access is unchanged; retained evidence is not erased.", schema([]string{"id", "target", "to"}, map[string]string{"id": "string", "target": "string", "to": "string"})},
		{"teams.get", "Read an invited team's brief and joined roster. Does not acknowledge messages or grant work authority.", schema([]string{"id"}, map[string]string{"id": "string", "target": "string"})},
		{"teams.join", "Join or rejoin an invited project team. Preserves native session, role credentials and inbox history.", schema([]string{"id"}, map[string]string{"id": "string"})},
		{"teams.leave", "Leave a team roster; invitation remains valid until revoked. Scoped work pauses and managed cancellation follows observation. Project-wide access is unchanged; retained task evidence remains.", schema([]string{"id"}, map[string]string{"id": "string"})},
	}
}
