// SPDX-License-Identifier: MPL-2.0

package runtime

type definitionSource struct{ Runtime, Kind, Directory string }

func definitionSources() []definitionSource {
	var sources []definitionSource
	for _, runtime := range []string{"claude", "codex", "agencyx"} {
		for _, kind := range []string{"agents", "skills", "commands"} {
			sources = append(sources, definitionSource{runtime, kind, "." + runtime + "/" + kind})
		}
	}
	return append(sources,
		definitionSource{"pi", "skills", ".pi/skills"},
		definitionSource{"pi", "commands", ".pi/prompts"},
		definitionSource{"hermes", "skills", ".hermes/skills"},
		definitionSource{"shared", "skills", ".agents/skills"},
	)
}
