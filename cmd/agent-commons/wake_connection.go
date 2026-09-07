// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"io"
	"path/filepath"
)

func openScopedWake(ctx context.Context, config, thread string) (*wakeWriter, projectConnection, error) {
	connection, err := openProjectConnection(config)
	if err != nil {
		return nil, connection, err
	}
	if err = connection.verifyIdentity(ctx); err != nil {
		return nil, connection, err
	}
	socket, err := filepath.EvalSymlinks(connection.config.Socket)
	if err != nil {
		return nil, connection, err
	}
	w, err := openWake(ctx, connection.config.State, thread, socket+"\x00"+connection.config.Identity, io.Discard)
	return w, connection, err
}
