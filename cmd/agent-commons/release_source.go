// SPDX-License-Identifier: MPL-2.0

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// updateRepo is the repository whose releases this binary updates from.
const updateRepo = "iksnae/agent-commons"

// maxDownload caps what a release may hand us. An archive holds a binary, the
// guides and a vendored source snapshot; anything past this is not a release.
const maxDownload = 256 << 20

// releaseSource locates published releases. The two hosts are fields rather
// than constants because GitHub serves the release metadata and the release
// files from different names, and because the entire download path has to be
// runnable against a local test server. scripts/install.sh shipped with its
// download path unexecuted; this one is exercised offline by the suite.
type releaseSource struct {
	repo         string
	apiBase      string
	downloadBase string
	client       *http.Client
}

func githubReleases() releaseSource {
	return releaseSource{
		repo:         updateRepo,
		apiBase:      "https://api.github.com",
		downloadBase: "https://github.com",
		client:       &http.Client{Timeout: 10 * time.Minute},
	}
}

// noReleaseError is what an operator sees before the first tag is cut. It names
// the route that still works, exactly as scripts/install.sh does.
func (s releaseSource) noReleaseError() error {
	return fmt.Errorf("no published release found for %s. GitHub's latest release excludes drafts and prereleases. "+
		"Build an archive yourself and install it with scripts/install.sh --archive PATH; see INSTALL.md", s.repo)
}

// latestTag reports the tag of the latest published release.
func (s releaseSource) latestTag(ctx context.Context) (string, error) {
	url := s.apiBase + "/repos/" + s.repo + "/releases/latest"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	response, err := s.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("cannot reach %s: %w", url, err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return "", s.noReleaseError()
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s answered %s", url, response.Status)
	}
	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		return "", fmt.Errorf("cannot read the release metadata from %s: %w", url, err)
	}
	if payload.TagName == "" {
		return "", s.noReleaseError()
	}
	return payload.TagName, nil
}

// download fetches one release file whole. Holding it in memory is what keeps
// the target directory clean until the archive has been checked.
func (s releaseSource) download(ctx context.Context, tag, file string) ([]byte, error) {
	url := s.downloadBase + "/" + s.repo + "/releases/download/" + tag + "/" + file
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("cannot reach %s: %w", url, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("release %s has no %s (%s). Nothing was replaced", tag, file, response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxDownload+1))
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", url, err)
	}
	if len(body) > maxDownload {
		return nil, fmt.Errorf("%s is larger than %d bytes. Refusing to install it", file, maxDownload)
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("downloaded %s is empty. Refusing to install it", file)
	}
	return body, nil
}

// verifyChecksum compares an archive against the sums published beside it.
//
// A checksum beside an archive detects transfer damage, not who published it.
// Agent Commons has no release signing yet, so a match is an intact download
// and nothing more.
func verifyChecksum(archive []byte, sums string, file string) error {
	expected := ""
	for _, line := range strings.Split(sums, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && (fields[1] == file || fields[1] == "*"+file) {
			expected = fields[0]
			break
		}
	}
	if expected == "" {
		return fmt.Errorf("SHA256SUMS has no entry for %s. Nothing was replaced", file)
	}
	digest := sha256.Sum256(archive)
	actual := hex.EncodeToString(digest[:])
	if !strings.EqualFold(expected, actual) {
		return fmt.Errorf("checksum mismatch for %s: expected %s, downloaded %s. "+
			"The copy was discarded and nothing was replaced", file, expected, actual)
	}
	return nil
}

// binaryFromArchive reads the one member it wants out of the release archive.
// No member name is ever used as a path on disk, so an archive cannot direct a
// write anywhere: the caller supplies the destination.
func binaryFromArchive(data []byte, directory string) ([]byte, error) {
	wanted := directory + "/agent-commons"
	decompressed, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("release archive is not readable: %w", err)
	}
	defer decompressed.Close()
	entries := tar.NewReader(decompressed)
	for {
		header, err := entries.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("release archive is not readable: %w", err)
		}
		if header.Name != wanted || header.Typeflag != tar.TypeReg {
			continue
		}
		body, err := io.ReadAll(io.LimitReader(entries, maxDownload+1))
		if err != nil {
			return nil, fmt.Errorf("cannot read %s from the release archive: %w", wanted, err)
		}
		if len(body) == 0 {
			return nil, fmt.Errorf("%s in the release archive is empty. Nothing was replaced", wanted)
		}
		return body, nil
	}
	return nil, fmt.Errorf("release archive does not contain %s. Nothing was replaced", wanted)
}
