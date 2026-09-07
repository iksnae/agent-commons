package main

import (
	"context"
	"encoding/json"
	"fmt"

	"agentcommons/internal/transport"
)

type rpcClient struct{ socket, token string }

func rpcCall[T any](ctx context.Context, client rpcClient, method string, params any) (T, error) {
	var result T
	request, err := json.Marshal(params)
	if err != nil {
		return result, err
	}
	response, err := transport.Call(ctx, client.socket, client.token, method, request)
	if err != nil {
		return result, fmt.Errorf("%s: %w", method, err)
	}
	err = json.Unmarshal(response, &result)
	return result, err
}
