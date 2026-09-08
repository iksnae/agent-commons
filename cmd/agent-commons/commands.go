// SPDX-License-Identifier: MPL-2.0

package main

import "strings"

// helpSection groups documented commands for the help screen. This catalog is
// the single source for the help screen, which a bare invocation also gets, so
// there is one list of commands and one rendering of it.
type helpSection struct {
	name string
	rows []string
}

func commandSections() []helpSection {
	return []helpSection{
		{name: "Connect a session", rows: []string{
			"init         set up a project: enroll or adopt a role, write project defaults",
			"enroll       create or adopt a project + role connection, no project defaults",
			"check-in     read onboarding, inbox and project context",
			"doctor       diagnose a connection without changing state",
			"retire       withdraw an identity, keeping its record and evidence",
			"reinstate    return a retired identity to service, with a new credential",
		}},
		{name: "Run the local service", rows: []string{
			"serve        run the local coordination service",
			"service      install or control the per-user service",
			"console      open the read-only command center",
		}},
		{name: "Harness integration", rows: []string{
			"connect-mcp  serve the scoped MCP connection",
			"harnesses    show runtime capabilities",
			"discover     inspect Claude or Codex project definitions",
			"inventory    list discovered project resources",
		}},
		{name: "Inspect and maintain", rows: []string{
			"bundle       install, verify or remove a local bundle",
			"watch        print unread inbox notifications as JSON",
			"call         invoke one RPC method (agents)",
			"methods      print the RPC catalog (agents)",
			"version      print the release this binary was built from",
			"update       replace this binary with the latest published release",
		}},
	}
}

// commandNames returns every documented command in help order.
func commandNames() []string {
	var names []string
	for _, section := range commandSections() {
		for _, row := range section.rows {
			if fields := strings.Fields(row); len(fields) > 0 {
				names = append(names, fields[0])
			}
		}
	}
	return names
}
