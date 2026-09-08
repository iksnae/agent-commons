// SPDX-License-Identifier: MPL-2.0

package main

import (
	"fmt"
	"io"
)

// devVersion is what an unstamped build calls itself. It is a fact, not a
// version number: a working-tree binary must never claim to be a release it was
// never cut from.
const devVersion = "dev"

// version names the release this binary was built from. Release archives are
// built with -buildvcs=false, so runtime/debug reports nothing about them;
// scripts/build-binaries.sh stamps this variable instead, with
// -ldflags "-X main.version=TAG" taken from the tag being built. An ordinary
// `go build` leaves it at devVersion.
var version = devVersion

func writeVersion(out io.Writer) error {
	_, err := fmt.Fprintf(out, "agent-commons %s\n", version)
	return err
}
