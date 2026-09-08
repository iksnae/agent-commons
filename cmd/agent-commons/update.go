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

	"agentcommons/internal/core"
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
		current:  core.Version,
		platform: runtime.GOOS + "-" + runtime.GOARCH,
		target:   target,
	}, out)
}

// performUpdate reports what is published, and replaces the target only after
// the download has been checked against the published sums.
// It has no --json flag and no report document: it emitted prose before this
// styling and emits the same prose now, so there is no machine contract here to
// preserve behind an escape flag.
func performUpdate(ctx context.Context, request updateRequest, out io.Writer) error {
	paint := painter{styled: writerIsStyled(out)}
	tag, err := request.source.latestTag(ctx)
	if err != nil {
		return err
	}
	if tag == request.current {
		_, err := fmt.Fprintf(out, "%s\n%s\n",
			paint.paint(headerStyle, fmt.Sprintf("agent-commons %s is the latest published release of %s.", tag, request.source.repo)),
			paint.paint(asideStyle, "Nothing was downloaded."))
		return err
	}
	// Ordering is GitHub's: "latest" is what it publishes as latest. This does
	// not compare version numbers, so it never claims a release is newer.
	if request.current == core.DevVersion {
		fmt.Fprintf(out, "%s\n", paint.paint(headerStyle, fmt.Sprintf(
			"This build carries no release stamp (%s), so it cannot be compared with %s.", core.DevVersion, tag)))
	} else {
		fmt.Fprintf(out, "%s\n", paint.paint(headerStyle, fmt.Sprintf(
			"This build is %s. The latest published release of %s is %s.", request.current, request.source.repo, tag)))
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
	fmt.Fprintf(out, "%s\n", paint.paint(asideStyle, fmt.Sprintf("Downloading %s (%s) …", file, tag)))
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
	fmt.Fprintf(out, "%s\n", paint.paint(asideStyle,
		"SHA256 checksum verified (damage in transfer only; not a publisher signature)."))
	binary, err := binaryFromArchive(archive, name)
	if err != nil {
		return err
	}
	if err := replaceBinary(staged, request.target, binary); err != nil {
		return err
	}
	fmt.Fprintf(out, "%s\n%s\n",
		paint.paint(headerStyle, fmt.Sprintf("Replaced %s with agent-commons %s.", request.target, tag)),
		paint.paint(asideStyle, "A running service keeps the old binary until it is restarted."))
	return nil
}

// replaceBinary writes the new binary beside the target and renames it over.
// The target is either the old binary or the whole new one; a half-written file
// is never left at that path, and a process already running from it keeps its
// own image rather than watching it change underneath.
//
// Two tests hold this, because the bytes end up in the right place either way
// and correct output alone would not notice a regression to a direct write:
// TestUpdateRenamesOverTheTargetRatherThanWritingIntoIt fails if this function
// writes into the target instead of renaming over it, and
// TestRunningExecutableSurvivesBeingRenamed proves the platform permits
// renaming a running executable rather than trusting that it does.
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
