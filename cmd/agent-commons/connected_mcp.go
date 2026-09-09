// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"

	"agentcommons/internal/transport"
)

func runConnectedMCP(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer) error {
	flags := flag.NewFlagSet("connect-mcp", flag.ContinueOnError)
	flags.SetOutput(errOut)
	config := flags.String("config", "", "private project-role connection file; falls back to AGENT_COMMONS_CONNECTION or an upward .agent-commons/project.json search")
	if err := parseFlags(flags, args, out); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("connect-mcp takes no positional arguments")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	resolvedConfig, err := resolveConnectionConfigPath(*config, cwd)
	if err != nil {
		return err
	}
	connection, err := openProjectConnection(resolvedConfig)
	if err != nil {
		return err
	}
	if err = connection.verifyIdentity(ctx); err != nil {
		return err
	}
	return transport.MCP(ctx, in, out, connection.client.socket, connection.client.token)
}
