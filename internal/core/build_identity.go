// SPDX-License-Identifier: MPL-2.0

package core

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"runtime/debug"
	"time"
)

// BuildIdentity reports which build is answering and when that process started.
// Without it a stale service and an unshipped feature are indistinguishable: a
// caller reading an absent method cannot tell whether this build lacks it or the
// running process predates it. Like SupervisorHealth these describe the running
// process, so they are computed once at construction and never persisted.
type BuildIdentity struct {
	// BinarySHA256 identifies a BUILD, never CODE: it changes on every commit
	// whether or not the compiled output changed, because a plain `go build`
	// stamps VCS revision, time and dirty-tree state into the binary by
	// default. Comparing digests answers "is this the same binary I released",
	// not "is this the same code" — reach for VCSRevision for that.
	BinarySHA256 string `json:"binarySha256,omitempty"`
	StartedAt    string `json:"startedAt"`
	VCSRevision  string `json:"vcsRevision,omitempty"`
	VCSTime      string `json:"vcsTime,omitempty"`
	// VCSModified is a pointer because a plain bool cannot distinguish "not
	// stamped" from "stamped, tree clean" — both would serialise as absent
	// under omitempty. nil means the binary carries no vcs.modified setting
	// (e.g. -buildvcs=false release builds); &true/&false means it does.
	VCSModified *bool `json:"vcsModified,omitempty"`
}

func newBuildIdentity(started time.Time) BuildIdentity {
	identity := BuildIdentity{BinarySHA256: executableDigest(), StartedAt: started.UTC().Format(time.RFC3339Nano)}
	if info, ok := debug.ReadBuildInfo(); ok {
		identity.VCSRevision, identity.VCSTime, identity.VCSModified = vcsIdentity(info.Settings)
	}
	return identity
}

// vcsIdentity extracts VCS stamping from build settings. Kept separate from
// debug.ReadBuildInfo so absence of vcs.* settings can be exercised directly,
// without relying on how this process happened to be built.
func vcsIdentity(settings []debug.BuildSetting) (revision, vcsTime string, modified *bool) {
	for _, setting := range settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.time":
			vcsTime = setting.Value
		case "vcs.modified":
			dirty := setting.Value == "true"
			modified = &dirty
		}
	}
	return revision, vcsTime, modified
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
