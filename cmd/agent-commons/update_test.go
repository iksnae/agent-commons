// SPDX-License-Identifier: MPL-2.0

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const (
	testRepo     = "iksnae/agent-commons"
	testPlatform = "darwin-arm64"
	oldBinary    = "the binary that is already installed"
	newBinary    = "the binary published in the release"
)

// fakeReleases stands in for GitHub. The whole download path runs against it,
// so the code that replaces an operator's binary is executed by the suite
// rather than first executed against a live release.
type fakeReleases struct {
	tag       string
	archive   []byte
	sums      string
	published bool
	downloads atomic.Int32
}

func (f *fakeReleases) start(t *testing.T) releaseSource {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/"+testRepo+"/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		if !f.published {
			http.NotFound(w, r)
			return
		}
		fmt.Fprintf(w, `{"tag_name":%q}`, f.tag)
	})
	mux.HandleFunc("/"+testRepo+"/releases/download/", func(w http.ResponseWriter, r *http.Request) {
		f.downloads.Add(1)
		switch path.Base(r.URL.Path) {
		case "agent-commons-" + testPlatform + ".tar.gz":
			w.Write(f.archive)
		case "SHA256SUMS":
			io.WriteString(w, f.sums)
		default:
			http.NotFound(w, r)
		}
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return releaseSource{repo: testRepo, apiBase: server.URL, downloadBase: server.URL, client: server.Client()}
}

// releaseArchive builds the distributable layout: one directory named for the
// platform, holding the binary.
func releaseArchive(t *testing.T, member, content string) []byte {
	t.Helper()
	var raw bytes.Buffer
	gz := gzip.NewWriter(&raw)
	archive := tar.NewWriter(gz)
	body := []byte(content)
	if member != "" {
		if err := archive.WriteHeader(&tar.Header{Name: member, Typeflag: tar.TypeReg, Mode: 0755, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := archive.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return raw.Bytes()
}

func sumsFor(archive []byte) string {
	digest := sha256.Sum256(archive)
	return hex.EncodeToString(digest[:]) + "  agent-commons-" + testPlatform + ".tar.gz\n"
}

// installedBinary writes a stand-in for the binary that is running and will be
// replaced. It never touches the operator's real installation.
func installedBinary(t *testing.T) string {
	t.Helper()
	target := filepath.Join(t.TempDir(), "agent-commons")
	if err := os.WriteFile(target, []byte(oldBinary), 0755); err != nil {
		t.Fatal(err)
	}
	return target
}

// assertUntouched proves the failure paths by inspecting the disk, not the
// error text: the original binary is byte-identical and nothing was staged
// beside it.
func assertUntouched(t *testing.T, target string) {
	t.Helper()
	body, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("original binary is gone: %v", err)
	}
	if string(body) != oldBinary {
		t.Fatalf("original binary changed to %q", body)
	}
	assertOnlyFile(t, target)
}

func assertOnlyFile(t *testing.T, target string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Dir(target))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != filepath.Base(target) {
		var names []string
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Fatalf("directory holds %v, not just %s", names, filepath.Base(target))
	}
}

func request(source releaseSource, current, target string) updateRequest {
	return updateRequest{source: source, current: current, platform: testPlatform, target: target}
}

func TestUpdateReportsCurrentReleaseWithoutDownloading(t *testing.T) {
	archive := releaseArchive(t, "agent-commons-"+testPlatform+"/agent-commons", newBinary)
	releases := &fakeReleases{tag: "v0.0.1", archive: archive, sums: sumsFor(archive), published: true}
	source := releases.start(t)
	target := installedBinary(t)

	var out bytes.Buffer
	if err := performUpdate(context.Background(), request(source, "v0.0.1", target), &out); err != nil {
		t.Fatal(err)
	}
	if got := releases.downloads.Load(); got != 0 {
		t.Fatalf("a current build fetched %d release files", got)
	}
	for _, want := range []string{"v0.0.1", "Nothing was downloaded"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("report omits %q: %s", want, out.String())
		}
	}
	assertUntouched(t, target)
}

func TestUpdateVerifiesThenReplacesTheBinary(t *testing.T) {
	archive := releaseArchive(t, "agent-commons-"+testPlatform+"/agent-commons", newBinary)
	releases := &fakeReleases{tag: "v0.0.2", archive: archive, sums: sumsFor(archive), published: true}
	source := releases.start(t)
	target := installedBinary(t)

	var out bytes.Buffer
	if err := performUpdate(context.Background(), request(source, "v0.0.1", target), &out); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != newBinary {
		t.Fatalf("binary was not replaced: %q", body)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0755 {
		t.Fatalf("replacement is mode %v", info.Mode().Perm())
	}
	// Nothing staged is left behind, and the report never overstates what a
	// checksum proves.
	assertOnlyFile(t, target)
	if !strings.Contains(out.String(), "not a publisher signature") {
		t.Fatalf("report claims more than a checksum proves: %s", out.String())
	}
	for _, want := range []string{"v0.0.2", target} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("report omits %q: %s", want, out.String())
		}
	}
}

func TestUpdateReplacesNothingWhenTheChecksumMismatches(t *testing.T) {
	archive := releaseArchive(t, "agent-commons-"+testPlatform+"/agent-commons", newBinary)
	damaged := append(append([]byte{}, archive...), "damage"...)
	releases := &fakeReleases{tag: "v0.0.2", archive: damaged, sums: sumsFor(archive), published: true}
	source := releases.start(t)
	target := installedBinary(t)

	err := performUpdate(context.Background(), request(source, "v0.0.1", target), &bytes.Buffer{})
	if err == nil {
		t.Fatal("a damaged archive was accepted")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("unclear refusal: %v", err)
	}
	assertUntouched(t, target)
}

func TestUpdateReplacesNothingWhenSumsHaveNoEntry(t *testing.T) {
	archive := releaseArchive(t, "agent-commons-"+testPlatform+"/agent-commons", newBinary)
	releases := &fakeReleases{tag: "v0.0.2", archive: archive, sums: "abc  some-other-file.tar.gz\n", published: true}
	source := releases.start(t)
	target := installedBinary(t)

	err := performUpdate(context.Background(), request(source, "v0.0.1", target), &bytes.Buffer{})
	if err == nil {
		t.Fatal("an unlisted archive was accepted")
	}
	if !strings.Contains(err.Error(), "SHA256SUMS has no entry") {
		t.Fatalf("unclear refusal: %v", err)
	}
	assertUntouched(t, target)
}

func TestUpdateReplacesNothingWhenTheArchiveLacksTheBinary(t *testing.T) {
	archive := releaseArchive(t, "agent-commons-"+testPlatform+"/README.md", "not a binary")
	releases := &fakeReleases{tag: "v0.0.2", archive: archive, sums: sumsFor(archive), published: true}
	source := releases.start(t)
	target := installedBinary(t)

	err := performUpdate(context.Background(), request(source, "v0.0.1", target), &bytes.Buffer{})
	if err == nil {
		t.Fatal("an archive without the binary was accepted")
	}
	if !strings.Contains(err.Error(), "does not contain") {
		t.Fatalf("unclear refusal: %v", err)
	}
	assertUntouched(t, target)
}

func TestUpdateFailsLoudlyWhenNoReleaseIsPublished(t *testing.T) {
	releases := &fakeReleases{published: false}
	source := releases.start(t)
	target := installedBinary(t)

	err := performUpdate(context.Background(), request(source, devVersion, target), &bytes.Buffer{})
	if err == nil {
		t.Fatal("an absent release was treated as success")
	}
	// The same vocabulary scripts/install.sh uses, including the route that
	// still works when nothing is published.
	for _, want := range []string{"no published release", testRepo, "--archive", "INSTALL.md"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("message omits %q: %v", want, err)
		}
	}
	if got := releases.downloads.Load(); got != 0 {
		t.Fatalf("fetched %d files with no release published", got)
	}
	assertUntouched(t, target)
}

func TestUpdateRefusesAnUnwritableTargetWithoutElevating(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write anywhere; the refusal cannot be exercised")
	}
	archive := releaseArchive(t, "agent-commons-"+testPlatform+"/agent-commons", newBinary)
	releases := &fakeReleases{tag: "v0.0.2", archive: archive, sums: sumsFor(archive), published: true}
	source := releases.start(t)
	target := installedBinary(t)
	dir := filepath.Dir(target)
	if err := os.Chmod(dir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0700) })

	err := performUpdate(context.Background(), request(source, "v0.0.1", target), &bytes.Buffer{})
	if err == nil {
		t.Fatal("an unwritable target was replaced")
	}
	if !strings.Contains(err.Error(), target) {
		t.Fatalf("refusal does not name the path: %v", err)
	}
	if strings.Contains(err.Error(), "sudo") && !strings.Contains(err.Error(), "never uses sudo") {
		t.Fatalf("refusal suggests elevation: %v", err)
	}
	if got := releases.downloads.Load(); got != 0 {
		t.Fatalf("downloaded %d files it could never install", got)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	assertUntouched(t, target)
}

const renameHelperEnv = "AGENT_COMMONS_RENAME_HELPER_MARKER"

// The atomic replacement renames a file that may be the running program.
// Renaming a running executable is documented to work on macOS and Linux;
// this proves it on the machine running the suite instead of trusting that.
func TestRunningExecutableSurvivesBeingRenamed(t *testing.T) {
	if marker := os.Getenv(renameHelperEnv); marker != "" {
		if err := os.WriteFile(marker, []byte("running"), 0600); err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Second)
		fmt.Println("helper finished after replacement")
		return
	}
	self, err := os.Executable()
	if err != nil {
		t.Skipf("cannot locate the test binary: %v", err)
	}
	dir := t.TempDir()
	helper := filepath.Join(dir, "helper")
	source, err := os.ReadFile(self)
	if err != nil {
		t.Skipf("cannot copy the test binary: %v", err)
	}
	if err := os.WriteFile(helper, source, 0755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "started")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, helper, "-test.run=^TestRunningExecutableSurvivesBeingRenamed$", "-test.count=1")
	cmd.Env = append(os.Environ(), renameHelperEnv+"="+marker)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if time.Now().After(deadline) {
			cmd.Process.Kill()
			cmd.Wait()
			t.Fatalf("helper never started: %s", output.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := os.Rename(helper, helper+".replaced"); err != nil {
		cmd.Process.Kill()
		cmd.Wait()
		t.Fatalf("cannot rename a running executable on this platform: %v", err)
	}
	if err := os.WriteFile(helper, []byte("a different binary now sits at this path"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("renamed process failed: %v\n%s", err, output.String())
	}
	if !strings.Contains(output.String(), "helper finished after replacement") {
		t.Fatalf("helper did not run to completion: %s", output.String())
	}
}

// A direct write over the live target passes every other test in this file:
// the bytes end up correct either way. What a running program needs is that its
// own open image is never the file being written. Hold a handle across the
// update, exactly as a running process holds one: a rename leaves this handle
// on the old file, while an in-place write truncates the same inode and the
// handle would read the replacement — or, mid-write, half of it.
func TestUpdateRenamesOverTheTargetRatherThanWritingIntoIt(t *testing.T) {
	archive := releaseArchive(t, "agent-commons-"+testPlatform+"/agent-commons", newBinary)
	releases := &fakeReleases{tag: "v0.0.2", archive: archive, sums: sumsFor(archive), published: true}
	source := releases.start(t)
	target := installedBinary(t)

	running, err := os.Open(target)
	if err != nil {
		t.Fatal(err)
	}
	defer running.Close()

	if err := performUpdate(context.Background(), request(source, "v0.0.1", target), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	held, err := io.ReadAll(running)
	if err != nil {
		t.Fatal(err)
	}
	if string(held) != oldBinary {
		t.Fatalf("the target was written in place: a handle opened before the update now reads %q, "+
			"so a running process would see its own image change under it", held)
	}
	replaced, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(replaced) != newBinary {
		t.Fatalf("the path does not hold the new binary: %q", replaced)
	}
}
