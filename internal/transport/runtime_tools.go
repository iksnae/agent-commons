// SPDX-License-Identifier: MPL-2.0

package transport

func runtimeTools() []tool {
	return []tool{{"runtime.status", "Read supervisor heartbeat and bounded durable queue counts. Agents see only their own identity; operator may filter sessionId or target. Limit 1..100, default 25. Canceled is a subset of failed. No provider availability claim or message content.", schema([]string{}, map[string]string{"sessionId": "string", "target": "string", "cursor": "string", "limit": "integer"})}}
}
