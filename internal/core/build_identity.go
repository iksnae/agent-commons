// SPDX-License-Identifier: MPL-2.0

package core

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"time"
)

// BuildIdentity reports which build is answering and when that process started.
// Without it a stale service and an unshipped feature are indistinguishable: a
// caller reading an absent method cannot tell whether this build lacks it or the
// running process predates it. Like SupervisorHealth these describe the running
// process, so they are computed once at construction and never persisted.
type BuildIdentity struct {
	BinarySHA256 string `json:"binarySha256,omitempty"`
	StartedAt    string `json:"startedAt"`
}

func newBuildIdentity(started time.Time) BuildIdentity {
	return BuildIdentity{BinarySHA256: executableDigest(), StartedAt: started.UTC().Format(time.RFC3339Nano)}
}

// executableDigest hashes the running binary so an operator can compare it with
// the checksums a release records. An unreadable executable yields an empty
// digest rather than refusing to start: build provenance is diagnostic and never
// a precondition for coordination.
func executableDigest() string {
	path, err := os.Executable()
	if err != nil {
		return ""
	}
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return ""
	}
	return hex.EncodeToString(digest.Sum(nil))
}
