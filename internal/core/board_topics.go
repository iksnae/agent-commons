// SPDX-License-Identifier: MPL-2.0

package core

// BoardTopics names contribution categories, not confidence or authority levels.
func BoardTopics() []string {
	return []string{"learning", "technique", "pitfall", "strategy", "idea", "experiment"}
}

func validBoardTopic(topic string) bool {
	for _, supported := range BoardTopics() {
		if topic == supported {
			return true
		}
	}
	return false
}
