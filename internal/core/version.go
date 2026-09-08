// SPDX-License-Identifier: MPL-2.0

package core

// DevVersion is what an unstamped build calls itself. It is a fact, not a
// version number: a working-tree binary must never claim to be a release it was
// never cut from.
const DevVersion = "dev"

// Version names the release this binary was built from. It lives here, beside
// the build identity that reports it, because two surfaces answer "which
// release is running" — `agent-commons version` and the runtime.status DTO —
// and two independently stamped copies of a version drift apart. One variable,
// read by both, cannot.
//
// Release archives are built with -buildvcs=false, so runtime/debug reports
// nothing about them and this stamp is the only thing a released binary can say
// about itself. scripts/build-ldflags.sh names this symbol in a -X flag, and
// both the packaging build and the offline rebuild that is compared against it
// take the flags from there, so the byte comparison cannot be defeated by the
// stamp itself. An ordinary `go build` leaves it at DevVersion.
var Version = DevVersion
