// SPDX-License-Identifier: MPL-2.0

package supervision

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

type NativeCommands struct{}

func (NativeCommands) Run(ctx context.Context, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, name, args...)
	command.WaitDelay = time.Second
	var output serviceOutput
	command.Stdout = &output
	command.Stderr = &output
	err := command.Run()
	if err != nil {
		return "", fmt.Errorf("%s: %w: %s", name, err, output.String())
	}
	if output.truncated {
		return "", fmt.Errorf("supervisor output exceeded 64 KiB; command may have completed")
	}
	return output.String(), nil
}

type serviceOutput struct {
	bytes.Buffer
	truncated bool
}

func (b *serviceOutput) Write(data []byte) (int, error) {
	n := len(data)
	remaining := (64 << 10) - b.Len()
	if len(data) > remaining {
		b.truncated = true
		data = data[:remaining]
	}
	_, _ = b.Buffer.Write(data)
	return n, nil
}
