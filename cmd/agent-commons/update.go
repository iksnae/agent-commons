// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

// updateRequest is everything the update needs from its environment, so the
// whole flow can run against a local server and a temporary file.
type updateRequest struct {
	source   releaseSource
	current  string
	platform string
	target   string
}

func runUpdate(ctx context.Context, args []string, out, errOut io.Writer) error {
	set := flag.NewFlagSet("update", flag.ContinueOnError)
	set.SetOutput(errOut)
	if err := set.Parse(args); err != nil {
		return err
	}
	if set.NArg() != 0 {
		return errors.New("update takes no arguments")
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	// Replace the file, not a symlink pointing at it.
	target, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return fmt.Errorf("cannot resolve the running binary at %s: %w", executable, err)
	}
	return performUpdate(ctx, updateRequest{
		source:   githubReleases(),
		current:  version,
		platform: runtime.GOOS + "-" + runtime.GOARCH,
		target:   target,
	}, out)
}

// performUpdate reports what is published, and replaces the target only after
// the download has been checked against the published sums.
func performUpdate(ctx context.Context, request updateRequest, out io.Writer) error {
	tag, err := request.source.latestTag(ctx)
	if err != nil {
		return err
	}
	if tag == request.current {
		_, err := fmt.Fprintf(out, "agent-commons %s is the latest published release of %s. Nothing was downloaded.\n",
			tag, request.source.repo)
		return err
	}
	// Ordering is GitHub's: "latest" is what it publishes as latest. This does
	// not compare version numbers, so it never claims a release is newer.
	if request.current == devVersion {
		fmt.Fprintf(out, "This build carries no release stamp (%s), so it cannot be compared with %s.\n",
			devVersion, tag)
	} else {
		fmt.Fprintf(out, "This build is %s. The latest published release of %s is %s.\n",
			request.current, request.source.repo, tag)
	}

	// Prove the destination is writable before spending a download on it, and
	// keep the staged file for the replacement itself.
	staged, err := os.CreateTemp(filepath.Dir(request.target), ".agent-commons-update-")
	if err != nil {
		return fmt.Errorf("cannot write beside %s, so it cannot be replaced: %w. "+
			"Make that directory writable yourself, or install elsewhere with "+
			"scripts/install.sh --to DIR. This command never uses sudo", request.target, err)
	}
	defer os.Remove(staged.Name())
	defer staged.Close()

	name := "agent-commons-" + request.platform
	file := name + ".tar.gz"
	fmt.Fprintf(out, "Downloading %s (%s) …\n", file, tag)
	archive, err := request.source.download(ctx, tag, file)
	if err != nil {
		return err
	}
	sums, err := request.source.download(ctx, tag, "SHA256SUMS")
	if err != nil {
		return err
	}
	if err := verifyChecksum(archive, string(sums), file); err != nil {
		return err
	}
	fmt.Fprintln(out, "SHA256 checksum verified (damage in transfer only; not a publisher signature).")
	binary, err := binaryFromArchive(archive, name)
	if err != nil {
		return err
	}
	if err := replaceBinary(staged, request.target, binary); err != nil {
		return err
	}
	fmt.Fprintf(out, "Replaced %s with agent-commons %s.\n", request.target, tag)
	fmt.Fprintln(out, "A running service keeps the old binary until it is restarted.")
	return nil
}

// replaceBinary writes the new binary beside the target and renames it over.
// The target is either the old binary or the whole new one; a half-written file
// is never left at that path. TestRunningExecutableSurvivesBeingRenamed proves
// the running program tolerates the rename rather than assuming it.
func replaceBinary(staged *os.File, target string, binary []byte) error {
	mode := fs.FileMode(0755)
	if info, err := os.Stat(target); err == nil {
		mode = info.Mode().Perm()
	}
	if _, err := staged.Write(binary); err != nil {
		return err
	}
	if err := staged.Sync(); err != nil {
		return err
	}
	if err := staged.Chmod(mode); err != nil {
		return err
	}
	if err := staged.Close(); err != nil {
		return err
	}
	if err := os.Rename(staged.Name(), target); err != nil {
		return fmt.Errorf("cannot replace %s: %w. The existing binary is unchanged", target, err)
	}
	return nil
}
