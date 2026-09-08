// SPDX-License-Identifier: MPL-2.0

package main

import (
	"fmt"
	"io"

	"agentcommons/internal/core"
)

// The release name lives in internal/core, beside the build identity that
// reports it over the wire, and the -ldflags stamp targets it there. This
// command surface reads that one variable rather than carrying a second copy,
// so `agent-commons version` and runtime.status cannot disagree about which
// release is running.
func writeVersion(out io.Writer) error {
	_, err := fmt.Fprintf(out, "agent-commons %s\n", core.Version)
	return err
}
