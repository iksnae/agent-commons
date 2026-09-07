// SPDX-License-Identifier: MPL-2.0

package codexrpc

import (
	"errors"
	"io"
)

type pipeStream struct {
	io.Reader
	io.Writer
	input  io.Closer
	output io.Closer
}

// Pipes joins process stdin/stdout. The caller still owns process termination and Wait.
func Pipes(input io.WriteCloser, output io.ReadCloser) io.ReadWriteCloser {
	return pipeStream{Reader: output, Writer: input, input: input, output: output}
}

func (p pipeStream) Close() error {
	return errors.Join(p.input.Close(), p.output.Close())
}
