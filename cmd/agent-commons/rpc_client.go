// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"

	"agentcommons/internal/transport"
)

// state is carried alongside the socket only so a failed dial can be
// classified: absence is evidenced by the state directory, and nothing else
// here needs it. It may be empty when --socket points somewhere with no
// matching state directory, which classification degrades on rather than
// rejects.
type rpcClient struct{ socket, token, state string }

// rpcCall is the seam every classified dial passes through. A transport failure
// is classified once, here, and returned typed; whoever the error reaches can
// then render it without re-deriving anything. An error that is not a
// reachability failure is returned exactly as before.
func rpcCall[T any](ctx context.Context, client rpcClient, method string, params any) (T, error) {
	var result T
	request, err := json.Marshal(params)
	if err != nil {
		return result, err
	}
	response, err := transport.Call(ctx, client.socket, client.token, method, request)
	if err != nil {
		if condition := classifyServiceCondition(client.state, err); condition != serviceConditionUnknown {
			return result, &serviceUnreachableError{condition: condition,
				socket: client.socket, state: client.state, err: err}
		}
		return result, fmt.Errorf("%s: %w", method, err)
	}
	err = json.Unmarshal(response, &result)
	return result, err
}
