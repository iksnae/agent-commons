// SPDX-License-Identifier: MPL-2.0

package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// Called only while core.New's exclusive state lock is held. Custom socket
// paths stay operator-managed; recovery is limited to our default endpoint.
func recoverServiceSocket(state, socket string) error {
	state, err := filepath.Abs(state)
	if err != nil {
		return err
	}
	socket, err = filepath.Abs(socket)
	if err != nil {
		return err
	}
	if socket != filepath.Join(state, "service.sock") {
		return nil
	}
	before, err := os.Lstat(socket)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	stat, ok := before.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Getuid()) || before.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("default socket must be a socket owned by the current user")
	}
	connection, err := net.DialTimeout("unix", socket, 200*time.Millisecond)
	if err == nil {
		connection.Close()
		return fmt.Errorf("default socket is active; refusing replacement")
	}
	if !errors.Is(err, syscall.ECONNREFUSED) {
		return fmt.Errorf("cannot establish that default socket is stale: %w", err)
	}
	after, err := os.Lstat(socket)
	if err != nil {
		return err
	}
	if !os.SameFile(before, after) {
		return fmt.Errorf("default socket changed during recovery")
	}
	return os.Remove(socket)
}
