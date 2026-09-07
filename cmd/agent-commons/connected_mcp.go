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
	config := flags.String("config", os.Getenv("AGENT_COMMONS_CONNECTION"), "private project-role connection file")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *config == "" || flags.NArg() != 0 {
		return errors.New("connect-mcp requires --config or AGENT_COMMONS_CONNECTION")
	}
	connection, err := openProjectConnection(*config)
	if err != nil {
		return err
	}
	if err = connection.verifyIdentity(ctx); err != nil {
		return err
	}
	return transport.MCP(ctx, in, out, connection.client.socket, connection.client.token)
}
