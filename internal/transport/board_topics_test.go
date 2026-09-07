// SPDX-License-Identifier: MPL-2.0

package transport

import (
	"reflect"
	"testing"

	"agentcommons/internal/core"
)

func TestBoardSchemasMatchAcceptedContributionTopics(t *testing.T) {
	count := 0
	for _, tool := range toolDefinitions() {
		if tool.Name != "board.post" && tool.Name != "board.list" {
			continue
		}
		count++
		properties := tool.InputSchema["properties"].(map[string]any)
		topic := properties["topic"].(map[string]any)
		if !reflect.DeepEqual(topic["enum"], core.BoardTopics()) {
			t.Fatal("schema differs from core topics", tool.Name)
		}
	}
	if count != 2 {
		t.Fatal("board topic schemas missing")
	}
}
